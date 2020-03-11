package gsm

// #cgo pkg-config: gammu-smsd
// #include <stdio.h>
// #include <gammu-smsd.h>
import "C"

import (
	"errors"
	"log"
)

var smsdStatus C.GSM_Error

func StartSMSD(config string, programName string) (err error) {

	var cfg *C.GSM_SMSDConfig

	name := C.CString(programName)
	cfg = C.SMSD_NewConfig(name)
	if cfg == nil {
		err = errors.New("Cannot create config")

		return
	}

	path := C.CString(config)
	e := C.SMSD_ReadConfig(path, cfg, C.gboolean(1))
	if e != ERR_NONE {
		log.Println("failed to read config")
		err = errors.New(errorString(int(e)))
		return
	}

	e = C.SMSD_MainLoop(cfg, C.gboolean(0), 0)
	if e != ERR_NONE {
		log.Println("Failed to run SMSD!")
		C.SMSD_FreeConfig(cfg)
		err = errors.New(errorString(int(e)))

		return
	}

	C.SMSD_FreeConfig(cfg)

	return nil
}
