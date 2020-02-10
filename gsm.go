// Author: Milan Nikolic <gen2brain@gmail.com>
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package gsm

// #cgo pkg-config: gammu
// #include <stdio.h>
// #include <gammu.h>
// extern void sendSMSCallback(GSM_StateMachine *sm, int status, int messageReference, void * user_data);
import "C"

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"
	"unsafe"
)

var smsSendStatus C.GSM_Error

const (
	ERR_NONE    = C.ERR_NONE
	ERR_UNKNOWN = C.ERR_UNKNOWN
	ERR_TIMEOUT = C.ERR_TIMEOUT
)

// Returns error message string
func errorString(e int) string {
	return C.GoString(C.GSM_ErrorString(C.GSM_Error(e)))
}

// Gammu GSM struct
type GSM struct {
	sm    *C.GSM_StateMachine
	modem *Modem
}

// Returns new GSM
func NewGSM() (g *GSM, err error) {
	g = &GSM{}
	g.sm = C.GSM_AllocStateMachine()

	if g.sm == nil {
		err = errors.New("Cannot allocate state machine")
	}

	return
}

// Enables global debugging to stderr
func (g *GSM) EnableDebug() {
	debugInfo := C.GSM_GetGlobalDebug()
	C.GSM_SetDebugFileDescriptor(C.stderr, C.gboolean(1), debugInfo)
	C.GSM_SetDebugLevel(C.CString("textall"), debugInfo)
}

// Connects to phone
func (g *GSM) Connect() (err error) {
	e := C.GSM_InitConnection(g.sm, 1) // 1 means number of replies to wait for
	if e != ERR_NONE {
		err = errors.New(errorString(int(e)))
	}

	// set callback for message sending
	C.GSM_SetSendSMSStatusCallback(g.sm, (C.SendSMSStatusCallback)(unsafe.Pointer(C.sendSMSCallback)), nil)
	return
}

// Reads configuration file
func (g *GSM) SetConfig(config string, section int) (err error) {
	path := C.CString(config)
	defer C.free(unsafe.Pointer(path))

	var cfg *C.INI_Section
	defer C.INI_Free(cfg)

	// find configuration file
	e := C.GSM_FindGammuRC(&cfg, path)
	if e != ERR_NONE {
		err = errors.New(errorString(int(e)))
		return
	}

	// read it
	e = C.GSM_ReadConfig(cfg, C.GSM_GetConfig(g.sm, 0), C.int(section))
	if e != ERR_NONE {
		err = errors.New(errorString(int(e)))
		return
	}

	// we have one valid configuration
	C.GSM_SetConfigNum(g.sm, 1)
	return
}

// Sends message
func (g *GSM) SendSMS(text, number string) (err error) {
	var sms C.GSM_SMSMessage
	var smsc C.GSM_SMSC

	sms.PDU = C.SMS_Submit                           // submit message
	sms.UDH.Type = C.UDH_NoUDH                       // no UDH, just a plain message
	sms.Coding = C.SMS_Coding_Default_No_Compression // default coding for text
	sms.Class = 1                                    // class 1 message (normal)

	C.EncodeUnicode((*C.uchar)(unsafe.Pointer(&sms.Text)), C.CString(text), C.ulong(len(text)))
	C.EncodeUnicode((*C.uchar)(unsafe.Pointer(&sms.Number)), C.CString(number), C.ulong(len(number)))

	// we need to know SMSC number
	smsc.Location = 1
	e := C.GSM_GetSMSC(g.sm, &smsc)
	if e != ERR_NONE {
		err = errors.New(errorString(int(e)))
		return
	}

	// set SMSC number in message
	sms.SMSC.Number = smsc.Number

	// Set flag before callind SendSMS, some phones might give instant response
	smsSendStatus = ERR_TIMEOUT

	// send message
	e = C.GSM_SendSMS(g.sm, &sms)
	if e != ERR_NONE {
		err = errors.New(errorString(int(e)))
		return
	}

	// wait for network reply
	for {
		C.GSM_ReadDevice(g.sm, C.gboolean(1))
		if smsSendStatus == ERR_NONE {
			break
		}
		if smsSendStatus != ERR_TIMEOUT {
			err = errors.New(errorString(int(smsSendStatus)))
			break
		}
	}

	return
}

func (g *GSM) SendLongSMS(text, number string) (err error) {
	var sms C.GSM_MultiSMSMessage
	var smsInfo C.GSM_MultiPartSMSInfo
	var smsc C.GSM_SMSC
	//var debugInfo *C.GSM_Debug_Info
	temp := text + text + "aa" //buffer size is 2*(len(text) + 1)
	bufferText := (*C.uchar)(unsafe.Pointer(C.CString(temp)))

	C.GSM_ClearMultiPartSMSInfo(&smsInfo)
	smsInfo.Class = 1
	smsInfo.EntriesNum = 1
	smsInfo.UnicodeCoding = C.gboolean(0)
	smsInfo.Entries[0].ID = C.SMS_ConcatenatedTextLong
	C.EncodeUnicode(bufferText, C.CString(text), C.ulong(len(text)))
	smsInfo.Entries[0].Buffer = bufferText

	e := C.GSM_EncodeMultiPartSMS(nil, &smsInfo, &sms)
	if e != ERR_NONE {
		err = errors.New(errorString(int(e)))
	}

	/*
		sms.PDU = C.SMS_Submit                           // submit message
		sms.UDH.Type = C.UDH_NoUDH                       // no UDH, just a plain message
		sms.Coding = C.SMS_Coding_Default_No_Compression // default coding for text
		sms.Class = 1                                    // class 1 message (normal)

		C.EncodeUnicode((*C.uchar)(unsafe.Pointer(&sms.Text)), C.CString(text), C.ulong(len(text)))
		C.EncodeUnicode((*C.uchar)(unsafe.Pointer(&sms.Number)), C.CString(number), C.ulong(len(number)))
	*/

	// we need to know SMSC number
	smsc.Location = 1
	e = C.GSM_GetSMSC(g.sm, &smsc)
	if e != ERR_NONE {
		err = errors.New(errorString(int(e)))
		return
	}

	for i := 0; i < int(sms.Number); i++ {
		// set SMSC number in message
		sms.SMS[i].SMSC.Number = smsc.Number

		C.EncodeUnicode((*C.uchar)(unsafe.Pointer(&sms.SMS[i].Number)), C.CString(number), C.ulong(len(number)))
		sms.SMS[i].PDU = C.SMS_Submit
		// Set flag before callind SendSMS, some phones might give instant response
		smsSendStatus = ERR_TIMEOUT

		// send message
		e = C.GSM_SendSMS(g.sm, &sms.SMS[i])
		if e != ERR_NONE {
			err = errors.New(errorString(int(e)))
			return
		}

		// wait for network reply
		for {
			C.GSM_ReadDevice(g.sm, C.gboolean(1))
			if smsSendStatus == ERR_NONE {
				break
			}
			if smsSendStatus != ERR_TIMEOUT {
				err = errors.New(errorString(int(smsSendStatus)))
				break
			}
		}
	}

	return
}

// Terminates connection and free memory
func (g *GSM) Terminate() (err error) {
	// terminate connection
	e := C.GSM_TerminateConnection(g.sm)
	if e != ERR_NONE {
		err = errors.New(errorString(int(e)))
	}

	// free up used memory
	C.GSM_FreeStateMachine(g.sm)
	if g.modem != nil {
		g.modem.close()
	}
	return
}

// Checks if phone is connected
func (g *GSM) IsConnected() bool {
	return int(C.GSM_IsConnected(g.sm)) != 0
}

// Callback for message sending
//export sendSMSCallback
func sendSMSCallback(sm *C.GSM_StateMachine, status C.int, messageReference C.int, user_data unsafe.Pointer) {
	t := fmt.Sprintf("Sent SMS on device %s - ", C.GoString(C.GSM_GetConfig(sm, -1).Device))
	if int(status) == 0 {
		log.Printf(t + "OK\n")
		smsSendStatus = ERR_NONE
	} else {
		log.Printf(t+"ERROR %d\n", int(status))
		smsSendStatus = ERR_UNKNOWN
	}
}

func (g *GSM) GetUSSDByCode(code string) (string, error) {
	deviceName := C.GoString(C.GSM_GetConfig(g.sm, -1).Device)
	isConnectedBefore := false
	if g.IsConnected() {
		isConnectedBefore = true
		e := C.GSM_TerminateConnection(g.sm)
		if e != ERR_NONE {
			return "", errors.New(errorString(int(e)))
		}
	}
	if !g.IsConnected() && isConnectedBefore {
		defer func() {
			err := g.Connect()
			if err != nil {
				fmt.Printf("Error: %v\n", err)
			}
		}()
	}
	if g.modem == nil {
		m, err := NewModem(deviceName)
		if err != nil {
			log.Printf("error open ussd port : %s", err.Error())
		}
		g.modem = m
	}
	if g.modem != nil {
		_, err := g.modem.SendCommand("AT+CSCS=\"GSM\"\r\n", true)
		if err != nil {
			log.Printf("error : %s", err.Error())
			return "", err
		}
		_, err = g.modem.SendCommand(fmt.Sprintf("AT+CUSD=1,\"%s\",15\r\n", code), true)
		if err != nil {
			log.Printf("error : %s", err.Error())
			return "", err
		}

		//time.Sleep(1 * time.Second)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		output, err := g.modem.ReadWithTimeout(ctx)
		if err != nil {
			return "", err
		}
		return output, nil
	}
	return "", nil
}
