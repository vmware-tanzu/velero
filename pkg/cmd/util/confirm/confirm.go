package confirm

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/pflag"
)

type ConfirmOptions struct {
	Confirm         bool
	flagDescription string
}

func NewConfirmOptions() *ConfirmOptions {
	return &ConfirmOptions{flagDescription: "Confirm action"}
}

func NewConfirmOptionsWithDescription(desc string) *ConfirmOptions {
	return &ConfirmOptions{flagDescription: desc}
}

// Bind confirm flags.
func (o *ConfirmOptions) BindFlags(flags *pflag.FlagSet) {
	flags.BoolVar(&o.Confirm, "confirm", o.Confirm, o.flagDescription)
}

// GetConfirmation ensures that the user confirms the action before proceeding.
func GetConfirmation(prompts ...string) bool {
	reader := bufio.NewReader(os.Stdin)

	for {
		for i := range prompts {
			fmt.Println(prompts[i])
		}
		fmt.Printf("Are you sure you want to continue (Y/N)? ")

		confirmation, err := reader.ReadString('\n')
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading user input: %v\n", err)
			return false
		}
		confirmation = strings.TrimSpace(confirmation)
		if len(confirmation) != 1 {
			continue
		}

		switch strings.ToLower(confirmation) {
		case "y":
			return true
		case "n":
			return false
		}
	}
}
