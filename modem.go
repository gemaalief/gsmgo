package gsm

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/tarm/serial"
)

const (
	baud    = 115200
	timeOut = 5 * time.Second
)

type Modem struct {
	Port *serial.Port
}

type respChan struct {
	result string
	err    error
}

func NewModem(deviceName string) (*Modem, error) {

	config := &serial.Config{Name: deviceName, Baud: baud, ReadTimeout: timeOut}
	con, err := serial.OpenPort(config)
	if err != nil {
		return nil, err
	}

	return &Modem{Port: con}, nil
}

func (m *Modem) Expect(possibilities []string) (string, error) {
	readMax := 2000
	for _, possibility := range possibilities {
		length := len(possibility)
		if length > readMax {
			readMax = length
		}
	}

	readMax = readMax + 2 // we need offset for \r\n sent by modem

	var status string = ""
	buf := make([]byte, readMax)

	for i := 0; i < readMax; i++ {
		// ignoring error as EOF raises error on Linux
		n, _ := m.Port.Read(buf)
		if n > 0 {
			status = string(buf[:n])

			for _, possibility := range possibilities {
				if strings.HasSuffix(status, possibility) {
					log.Println("--- Expect:", m.transposeLog(strings.Join(possibilities, "|")), "Got:", m.transposeLog(status))
					return status, nil
				}
			}
		}
	}

	log.Println("--- Expect:", m.transposeLog(strings.Join(possibilities, "|")), "Got:", m.transposeLog(status), "(match not found!)")
	return status, errors.New("match not found")
}

func (m *Modem) Send(command string) {
	log.Println("--- Send:", m.transposeLog(command))
	m.Port.Flush()
	_, err := m.Port.Write([]byte(command))
	if err != nil {
		log.Fatal(err)
	}
}

func (m *Modem) Read() (string, error) {
	var output string = ""
	scanner := bufio.NewScanner(m.Port)
	for scanner.Scan() {
		output += scanner.Text()
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return output, nil
}

func (m *Modem) ReadWithTimeout(ctx context.Context) (string, error) {
	select {
	case <-ctx.Done():
		return "", fmt.Errorf("context timeout, ran out of time")
	case respChan := <-m.ReadWithContext(ctx):
		return respChan.result, respChan.err
	}
}

func (m *Modem) ReadWithContext(ctx context.Context) <-chan *respChan {
	r := make(chan *respChan, 1)
	go func() {
		result, err := m.Read()
		if err != nil {
			r <- &respChan{"", err}
			return
		}
		if !strings.HasSuffix(result, "\"Terima") {
			r <- &respChan{result, nil}
		}
		return
	}()
	return r
}

func (m *Modem) SendCommand(command string, waitForOk bool) (string, error) {
	m.Send(command)

	if waitForOk {
		output, err := m.Expect([]string{"OK\r\n", "ERROR\r\n"}) // we will not change api so errors are ignored for now
		return output, err
	} else {
		return m.Read()
	}
}

func (m *Modem) transposeLog(input string) string {
	output := strings.Replace(input, "\r\n", "\\r\\n", -1)
	return strings.Replace(output, "\r", "\\r", -1)
}

func (m *Modem) close() error {
	return m.Port.Close()
}
