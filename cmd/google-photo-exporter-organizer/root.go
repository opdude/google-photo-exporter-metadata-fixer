package cmd

import (
	"github.com/spf13/cobra"

	"github.com/opdude/google-photo-exporter-metadata-fixer/internal/metadata_fixer"
)

var (
	rootCmd = &cobra.Command{
		Use:   "google-exporter-metadata-fixer",
		Short: "google-exporter-metadata-fixer is a CLI tool to fix photo metadata from a google photo export",
	}
)

// Execute executes the root command.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	var removeJSONFiles bool
	var organizeCmd = &cobra.Command{
		Use:   "fix",
		Short: "Fix photo metadata from a google photo export",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return metadata_fixer.FixPhotoMetadata(args[0], removeJSONFiles)
		},
	}

	organizeCmd.Flags().BoolVarP(&removeJSONFiles, "deleteJSONFiles", "d", false, "Delete the JSON files after processing")

	rootCmd.AddCommand(organizeCmd)
}
