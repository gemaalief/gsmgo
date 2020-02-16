package main

import (
	"flag"
	"fmt"
	"os"
	"os/user"
	"path/filepath"

	gsm "github.com/gemaalief/gsmgo"
)

func main() {
	cfg := flag.String("config", "", "Config file")
	debug := flag.Bool("debug", false, "Enable debugging")
	mode := flag.String("mode", "sms", "select mode : [sms | ussd]")
	code := flag.String("code", "", "ussd code")
	text := flag.String("text", "", "Text Message")
	number := flag.String("number", "", "Phone Number")
	sectionPtr := flag.Int("section", 0, "called gammu section")
	flag.Parse()

	if *mode == "sms" {
		if *text == "" || *number == "" {
			flag.Usage()
			os.Exit(1)
		}

		if len(*text) > 160 {
			fmt.Println("Message exceeds 160 characters")
			os.Exit(1)
		}
	} else if *mode == "ussd" {
		if *code == "" {
			flag.Usage()
			os.Exit(1)
		}
	}

	g, err := gsm.NewGSM()
	if err != nil {
		fmt.Println(err)
	}
	defer g.Terminate()

	if *debug {
		g.EnableDebug()
	}

	usr, _ := user.Current()
	homedir := usr.HomeDir

	dir, err := filepath.Abs(filepath.Dir(os.Args[0]))
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}

	var config string
	if *cfg != "" {
		config = *cfg
	} else if _, err := os.Stat("/etc/gsmgo.conf"); err == nil {
		config = "/etc/gsmgo.conf"
	} else if _, err := os.Stat(filepath.Join(homedir, ".gsmgo.conf")); err == nil {
		config = filepath.Join(homedir, ".gsmgo.conf")
	} else if _, err := os.Stat(filepath.Join(dir, "gsmgo.conf")); err == nil {
		config = filepath.Join(dir, "gsmgo.conf")
	} else {
		fmt.Println("Error: Config file not found")
		os.Exit(1)
	}

	var section int
	if *sectionPtr != 0 {
		section = *sectionPtr
	}

	err = g.SetConfig(config, section)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}

	err = g.Connect()
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	}

	if !g.IsConnected() {
		fmt.Println("Phone is not connected")
		os.Exit(1)
	}

	if *mode == "sms" {
		err = g.SendSMS(*text, *number)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
		}
	} else if *mode == "ussd" {
		result, err := g.GetUSSDByCode(*code, "")
		if err != nil {
			fmt.Printf("Error: %v\n", err)
		}
		fmt.Println(result)
	} else if *mode == "read" {
		result, err := g.ReadSMS(true)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
		}
		for _, msg := range result {
			fmt.Printf("%s : %s\n", msg.Number, msg.Text)
		}
	} else if *mode == "receive" {
		cb := func(tes, tes2 string) error {
			fmt.Println(tes + " - " + tes2)
			return nil
		}
		g.SetCallBack(cb)
		err := g.WaitForSMS(1)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
		}
		g.AlwaysReadUntilBreak()
	}
}
