/*
Copyright © 2024 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

// entryCmd represents the entry command
var entryCmd = &cobra.Command{
	Use:   "entry",
	Short: "A brief description of your command",
	Long:  ``,
	Run: func(cmd *cobra.Command, args []string) {
		if len(os.Getenv("DEBUG")) > 0 {
			f, err := tea.LogToFile("entry.log", "main")
			if err != nil {
				fmt.Println("fatal:", err)
				os.Exit(1)
			}
			defer f.Close()
		}
		p := tea.NewProgram(NewMainModel(), tea.WithAltScreen())
		if _, err := p.Run(); err != nil {
			fmt.Println("could not start program:", err)
		}

	},
}

func init() {
	rootCmd.AddCommand(entryCmd)
}

// General stuff for styling the view
var (
	keywordStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("211"))
	subtleStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	ticksStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("79"))
	checkboxStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	mainStyle     = lipgloss.NewStyle().MarginLeft(2)
)

type sessionState int

const (
	filepickView sessionState = iota
	buildView
)

type MainModel struct {
	rootModel  tea.Model
	ActiveFile string

	Loaded   bool
	Quitting bool
}

func NewMainModel() MainModel {
	var newrootModel tea.Model

	pickModel := NewPickModel()
	newrootModel = &pickModel
	return MainModel{
		rootModel: newrootModel,
	}

}

func (m MainModel) Init() tea.Cmd {
	return m.rootModel.Init()
}

// Main update function. Handle state
func (m MainModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return m.rootModel.Update(msg)
}

// The main view, which just calls the appropriate sub-view
func (m MainModel) View() string {
	return m.rootModel.View()
}

func (m MainModel) SwitchScreen(model tea.Model) (tea.Model, tea.Cmd) {
	m.rootModel = model
	return m, m.rootModel.Init()
}
