/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/filepicker"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

// pickCmd represents the pick command
var pickCmd = &cobra.Command{
	Use:   "pick",
	Short: "A brief description of your command",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		pickmain()
	},
}

func init() {
	rootCmd.AddCommand(pickCmd)
}

type PickModel struct {
	filepicker   filepicker.Model
	selectedFile string
	quitting     bool
	err          error
}

type clearErrorMsg struct{}

func clearErrorAfter(t time.Duration) tea.Cmd {
	return tea.Tick(t, func(_ time.Time) tea.Msg {
		return clearErrorMsg{}
	})
}

func (m PickModel) Init() tea.Cmd {
	return m.filepicker.Init()
}

func (m PickModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.quitting = true
			return m, tea.Quit
		case "o":
			log.Println("Selected file: " + m.selectedFile)

			newBuildModel := NewBuildModel("Jenkinsfile")
			rootModel := NewMainModel()
			return rootModel.SwitchScreen(newBuildModel)
		}
	case clearErrorMsg:
		m.err = nil
	}

	var cmd tea.Cmd
	m.filepicker, cmd = m.filepicker.Update(msg)

	// Did the user select a file?
	if didSelect, path := m.filepicker.DidSelectFile(msg); didSelect {
		// Get the path of the selected file.
		m.selectedFile = path
	}

	// Did the user select a disabled file?
	// This is only necessary to display an error to the user.
	if didSelect, path := m.filepicker.DidSelectDisabledFile(msg); didSelect {
		// Let's clear the selectedFile and display an error.
		m.err = errors.New(path + " is not valid.")
		m.selectedFile = ""
		return m, tea.Batch(cmd, clearErrorAfter(2*time.Second))
	}

	return m, cmd
}

func (m PickModel) View() string {
	if m.quitting {
		return ""
	}
	var s strings.Builder
	s.WriteString("\n  ")
	if m.err != nil {
		s.WriteString(m.filepicker.Styles.DisabledFile.Render(m.err.Error()))
	} else if m.selectedFile == "" {
		s.WriteString("Pick a file:")
	} else {
		s.WriteString("Selected file: " + m.filepicker.Styles.Selected.Render(m.selectedFile))
	}
	s.WriteString("\n\n" + m.filepicker.View() + "\n")
	return s.String()
}

func NewPickModel() PickModel {
	fp := filepicker.New()
	fp.AllowedTypes = []string{".groovy", ".gvy", "Jenkinsfile"}
	fp.CurrentDirectory, _ = os.Getwd()

	return PickModel{
		filepicker: fp,
	}

}

func pickmain() {
	if len(os.Getenv("DEBUG")) > 0 {
		f, err := tea.LogToFile("picker.log", "filepicker")
		if err != nil {
			fmt.Println("fatal:", err)
			os.Exit(1)
		}
		defer f.Close()
	}

	m := NewPickModel()
	tm, _ := tea.NewProgram(&m).Run()
	mm := tm.(PickModel)
	log.Println("\n  You selected: " + m.filepicker.Styles.Selected.Render(mm.selectedFile) + "\n")
	relativePath := strings.Replace(mm.selectedFile, mm.filepicker.CurrentDirectory+"/", "", 1)
	log.Println("\n  Relative path: " + m.filepicker.Styles.Selected.Render(relativePath) + "\n")

}
