package util

import "github.com/spf13/cobra"

func CheckError(err error) {
	// For now just delegate to Cobra's CheckErr
	cobra.CheckErr(err)
}
