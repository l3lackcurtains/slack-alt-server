// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/mattermost/mattermost-server/v5/store"
	"github.com/spf13/cobra"
)

var IntegrityCmd = &cobra.Command{
	Use:   "integrity",
	Short: "Check database data integrity",
	RunE:  integrityCmdF,
}

func init() {
	IntegrityCmd.Flags().Bool("confirm", false, "Confirm you really want to run a complete integrity check that may temporarily harm system performance")
	IntegrityCmd.Flags().BoolP("verbose", "v", false, "Show detailed information on integrity check results")
	RootCmd.AddCommand(IntegrityCmd)
}

func printRelationalIntegrityCheckResult(data store.RelationalIntegrityCheckData, verbose bool) {
	fmt.Println(fmt.Sprintf("Found %d records in relation %s orphans of relation %s",
		len(data.Records), data.ChildName, data.ParentName))
	if !verbose {
		return
	}
	for _, record := range data.Records {
		var parentId string

		if record.ParentId == nil {
			parentId = "NULL"
		} else if *record.ParentId == "" {
			parentId = "empty"
		} else {
			parentId = *record.ParentId
		}

		if record.ChildId != nil {
			if parentId == "NULL" || parentId == "empty" {
				fmt.Println(fmt.Sprintf("  Child %s (%s.%s) has %s ParentIdAttr (%s.%s)", *record.ChildId, data.ChildName, data.ChildIdAttr, parentId, data.ChildName, data.ParentIdAttr))
			} else {
				fmt.Println(fmt.Sprintf("  Child %s (%s.%s) is missing Parent %s (%s.%s)", *record.ChildId, data.ChildName, data.ChildIdAttr, parentId, data.ChildName, data.ParentIdAttr))
			}
		} else {
			if parentId == "NULL" || parentId == "empty" {
				fmt.Println(fmt.Sprintf("  Child has %s ParentIdAttr (%s.%s)", parentId, data.ChildName, data.ParentIdAttr))
			} else {
				fmt.Println(fmt.Sprintf("  Child is missing Parent %s (%s.%s)", parentId, data.ChildName, data.ParentIdAttr))
			}
		}
	}
}

func printIntegrityCheckResult(result store.IntegrityCheckResult, verbose bool) {
	switch data := result.Data.(type) {
	case store.RelationalIntegrityCheckData:
		printRelationalIntegrityCheckResult(data, verbose)
	}
}

func integrityCmdF(command *cobra.Command, args []string) error {
	a, err := InitDBCommandContextCobra(command)
	if err != nil {
		return err
	}
	defer a.Shutdown()

	confirmFlag, _ := command.Flags().GetBool("confirm")
	if !confirmFlag {
		var confirm string
		fmt.Fprintf(os.Stdout, "This check may harm performance on live systems. Are you sure you want to proceed? (y/N): ")
		fmt.Scanln(&confirm)
		if !strings.EqualFold(confirm, "y") && !strings.EqualFold(confirm, "yes") {
			fmt.Fprintf(os.Stderr, "Aborted.\n")
			return nil
		}
	}

	verboseFlag, _ := command.Flags().GetBool("verbose")
	results := a.Srv().Store.CheckIntegrity()
	for result := range results {
		if result.Err != nil {
			fmt.Fprintf(os.Stderr, "%s\n", result.Err.Error())
			break
		}
		printIntegrityCheckResult(result, verboseFlag)
	}

	return nil
}
