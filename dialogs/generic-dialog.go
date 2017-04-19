package dialogs

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"strconv"

	log "github.com/Sirupsen/logrus"
)

var Handler DialogHandler

const Retries = 5

type DialogHandler struct {
	Reader io.Reader
}

func (d *DialogHandler) GetRead() io.Reader {
	if d.Reader == nil {
		d.Reader = os.Stdin
	}
	return d.Reader
}

func GetSingleAnswer(question string, validators ...ValidatorFn) string {
	reader := bufio.NewReader(Handler.GetRead())
	retries := Retries
	fmt.Print("[?] ", question)

Loop:
	for retries > 0 {
		retries--

		inp, err := reader.ReadString('\n')
		if err != nil {
			log.Error(err.Error())
			fmt.Println("[-] Could not read input string from stdin: ", err.Error())
			fmt.Print("[?] Please repeat: ")
			continue
		}

		inp = strings.TrimSpace(inp)

		for _, validator := range validators {
			if !validator(inp) {
				continue Loop
			}
		}

		return inp
	}

	fmt.Println("\n[-] You have reached maximum number of retries")
	os.Exit(3)

	return ""
}

func GetSingleNumber(question string, validators ...NumberValidatorFn) int {
	reader := bufio.NewReader(Handler.GetRead())
	retries := Retries
	fmt.Print("[?] ", question)

Loop:
	for retries > 0 {
		retries--

		inp, err := reader.ReadString('\n')
		if err != nil {
			log.Error(err.Error())
			fmt.Println("[-] Could not read input string from stdin: ", err.Error())
			fmt.Print("[?] Please repeat: ")
			continue
		}

		num, err := strconv.Atoi(inp)
		if err != nil {
			log.Error(err.Error())
			fmt.Println("[-] Invalid input: ", err.Error())
			fmt.Print("[?] Please repeat: ")
			continue
		}

		for _, validator := range validators {
			if !validator(num) {
				continue Loop
			}
		}

		return num
	}

	fmt.Println("\n[-] You have reached maximum number of retries")
	os.Exit(3)

	return 0
}

func YesNoDialog(question string) bool {
	answer := GetSingleAnswer(question+" (\x1b[33my/yes\x1b[0m OR \x1b[33mn/no\x1b[0m):", YesNoValidator)
	return strings.EqualFold(answer, "y") || strings.EqualFold(answer, "yes")
}

func SelectOneDialog(question string, opts []string) int {
	reader := bufio.NewReader(Handler.GetRead())
	retries := Retries

	for i, v := range opts {
		fmt.Printf("   \x1b[33m[%d]\x1b[0m %s\n", i+1, v)
	}

	for retries > 0 {
		retries--
		fmt.Print("[?] ", question)

		answer, err := reader.ReadString('\n')
		if err != nil {
			log.Error(err.Error())
			fmt.Println("[-] Could not read input string from stdin: ", err.Error())
			continue
		}

		inp, err := strconv.Atoi(strings.TrimSpace(answer))
		if err != nil || inp < 1 || inp > len(opts) {
			fmt.Println("[-] Invalid user input, ", err, " please repeat: ")
			continue
		}

		return inp - 1
	}

	fmt.Println("\n[-] You reached maximum number of retries")
	os.Exit(3)
	return 0
}
