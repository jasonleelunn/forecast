package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type model struct {
	table  table.Model
	status int
	body   string
	err    error
}

type location struct {
	Id     string `json:"id"`
	Name   string `json:"name"`
	Region string `json:"region"`
}

type locations struct {
	Location []location `json:"location"`
}

type Rows []table.Row

func (rows Rows) Len() int {
	return len(rows)
}

func (rows Rows) Less(i, j int) bool {
	// Sort by Name field alphabetically
	return rows[i][0] < rows[j][0]
}

func (rows Rows) Swap(i, j int) {
	rows[i], rows[j] = rows[j], rows[i]
}

var baseStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("240"))

const baseUrl = "http://datapoint.metoffice.gov.uk/public/data/"

func main() {
	endpoint := "val/wxfcs/all/json/sitelist"

	url := makeUrl(endpoint)

	rows := fetchData(url)

	if rows == nil {
		log.Fatal("Could not fetch row data.")
	}

	columns := []table.Column{
		{Title: "Name", Width: 30},
		{Title: "ID", Width: 10},
		{Title: "Region", Width: 10},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(20),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(s)

	m := model{table: t}
	p := tea.NewProgram(m)

	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {

	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			if m.table.Focused() {
				m.table.Blur()
			} else {
				m.table.Focus()
			}
		case "q", "ctrl+c":
			return m, tea.Quit
		case "enter":
			return m, tea.Batch(
				tea.Printf("Let's go to %s!", m.table.SelectedRow()[0]),
			)
		}
	}

	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m model) View() string {
	return baseStyle.Render(m.table.View()) + "\n"
}

func makeUrl(endpoint string) string {
	apiKey := os.Getenv("MET_OFFICE_API_KEY")
	return baseUrl + endpoint + "?key=" + apiKey
}

func fetchData(url string) Rows {
	c := &http.Client{
		Timeout: 10 * time.Second,
	}

	res, err := c.Get(url)

	if err != nil {
		fmt.Println("Error fetching endpoint:", err)
		return nil
	}

	defer res.Body.Close() // nolint:errcheck	// Read the response body

	body, err := io.ReadAll(res.Body)
	if err != nil {
		fmt.Println("Error reading body:", err)
		return nil
	}

	var data struct {
		Locations locations `json:"locations"`
	}

	err = json.Unmarshal(body, &data)
	if err != nil {
		fmt.Println("Error decoding JSON:", err)
		return nil
	}

	var rows Rows

	for _, location := range data.Locations.Location {
		rows = append(rows, table.Row{location.Name, location.Id, location.Region})
	}

	sort.Sort(rows)

	return rows
}
