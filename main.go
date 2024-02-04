package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"slices"
	"sort"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jasonleelunn/forecast/internal/data"
	"github.com/lithammer/fuzzysearch/fuzzy"
)

type model struct {
	err            error
	textInput      textinput.Model
	table          table.Model
	list           list.Model
	siteData       data.SiteData
	locationChosen bool
	forecastChosen bool
}

type location struct {
	Id     string `json:"id"`
	Name   string `json:"name"`
	Region string `json:"region"`
}

type locations struct {
	Location []location `json:"location"`
}

type forecastItem struct {
	title, desc string
}

func (i forecastItem) Title() string       { return i.title }
func (i forecastItem) Description() string { return i.desc }
func (i forecastItem) FilterValue() string { return i.title }

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

const baseUrl = "http://datapoint.metoffice.gov.uk/public/data/"

var apiKey = os.Getenv("MET_OFFICE_API_KEY")

var (
	baseStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("240"))

	listStyle = lipgloss.NewStyle().Margin(1, 2)
)

var placenames []string
var rows Rows

func makeUrl(endpoint string, paramList ...string) string {
	params := ""
	for _, param := range paramList {
		params += "&" + param
	}

	return baseUrl + endpoint + "?key=" + apiKey + params
}

func fetchData(url string) []byte {
	c := &http.Client{
		Timeout: 10 * time.Second,
	}

	res, err := c.Get(url)

	if err != nil {
		fmt.Println("Error fetching endpoint:", err)
		return nil
	}

	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		fmt.Println("Error reading body:", err)
		return nil
	}

	return body
}

func extractRows(body []byte) Rows {
	var data struct {
		Locations locations `json:"locations"`
	}

	err := json.Unmarshal(body, &data)
	if err != nil {
		fmt.Println("Error decoding JSON:", err)
		return nil
	}

	for _, location := range data.Locations.Location {
		placenames = append(placenames, location.Name)
		rows = append(rows, table.Row{location.Name, location.Id, location.Region})
	}

	slices.Sort(placenames)
	sort.Sort(rows)

	return rows
}

func setupTable(rows Rows) table.Model {
	columns := []table.Column{
		{Title: "Name", Width: 40},
		{Title: "ID", Width: 10},
		{Title: "Region", Width: 10},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(false),
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

	return t
}

func setupTextInput() textinput.Model {
	ti := textinput.New()
	ti.Placeholder = "Search for a placename"
	ti.Focus()
	ti.CharLimit = 156
	ti.Width = 60

	return ti
}

func setupList() list.Model {
	li := list.New(nil, list.NewDefaultDelegate(), 0, 0)
	li.SetFilteringEnabled(false)
	li.SetShowTitle(true)
	li.SetShowStatusBar(false)

	return li
}

func initialModel() model {
	endpoint := "val/wxfcs/all/json/sitelist"
	url := makeUrl(endpoint)
	res := fetchData(url)
	if res == nil {
		log.Fatal("Could not fetch sitelist data.")
	}

	rows := extractRows(res)

	t := setupTable(rows)
	ti := setupTextInput()
	li := setupList()

	return model{
		textInput: ti,
		table:     t,
		list:      li,
	}
}

func getSiteData(siteId string) data.SiteData {
	endpoint := "val/wxfcs/all/json/" + siteId
	url := makeUrl(endpoint, "res=daily")
	res := fetchData(url)
	if res == nil {
		log.Fatal("Could not fetch site data.")
	}

	var siteData data.SiteData

	err := json.Unmarshal(res, &siteData)
	if err != nil {
		log.Fatal("Error decoding JSON:", err)
	}

	return siteData
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		h, v := listStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)
	}

	if m.forecastChosen {
		return updateForecast(msg, m)
	} else if m.locationChosen {
		return updateLocation(msg, m)
	} else {
		return updateSearch(msg, m)
	}
}

func updateSearch(msg tea.Msg, m model) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	var textInputCmd tea.Cmd
	var tableCmd tea.Cmd

	m.textInput, textInputCmd = m.textInput.Update(msg)
	m.table, tableCmd = m.table.Update(msg)

	cmds = append(cmds, textInputCmd, tableCmd)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			if m.textInput.Focused() {
				m.textInput.Blur()
				m.table.Focus()
			} else if m.table.Focused() {
				id := m.table.SelectedRow()[1]

				m.siteData = getSiteData(id)

				var forecasts []list.Item

				for _, period := range m.siteData.Site.Info.Location.Periods {
					for _, forecast := range period.Forecasts {
						code := forecast.WeatherCode

						date, err := time.Parse("2006-01-02Z", period.Date)
						if err != nil {
							log.Fatal("Failed to parse date", err)
						}

						item := forecastItem{title: date.Format("Mon, 02 Jan 2006") + " (" + forecast.Time + ")", desc: data.WeatherCodes[code]}

						forecasts = append(forecasts, item)
					}
				}

				m.locationChosen = true

				cmd := m.list.SetItems(forecasts)
				m.list.Title = m.siteData.Site.Info.Location.Name + ", " + m.siteData.Site.Info.Location.Country

				return m, cmd
			}
		case "esc":
			if m.table.Focused() {
				m.table.Blur()
				m.textInput.Focus()
			}
		default:
			input := m.textInput.Value()

			if len(input) > 0 {
				matchedNames := fuzzy.RankFindFold(input, placenames)
				sort.Sort(matchedNames)

				var filteredRows Rows

				for _, rankedMatch := range matchedNames {
					index := rankedMatch.OriginalIndex
					filteredRows = append(filteredRows, rows[index])
				}

				m.table.SetRows(filteredRows)
			} else {
				m.table.SetRows(rows)
			}

		}
	}

	return m, tea.Batch(cmds...)
}

func updateLocation(msg tea.Msg, m model) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			// index := m.list.Index()

			m.forecastChosen = true
		}
	}

	var cmd tea.Cmd

	m.list, cmd = m.list.Update(msg)

	return m, cmd
}

func updateForecast(msg tea.Msg, m model) (tea.Model, tea.Cmd) { return m, nil }

func (m model) View() string {
	var s string

	if m.forecastChosen {
		s += forecastView(m)
	} else if m.locationChosen {
		s += locationView(m)
	} else {
		s += searchView(m)
	}

	return s + "\n(ctrl+c to quit)\n"
}

func searchView(m model) string {
	return baseStyle.Render(m.textInput.View()) + "\n" + baseStyle.Render(m.table.View())
}

func locationView(m model) string {
	return listStyle.Render(m.list.View())
}

func forecastView(m model) string {
	return listStyle.Render(m.siteData.Site.Info.Location.Periods[0].Forecasts[0].Precipitation + "% chance of rain")
}

func main() {
	m := initialModel()
	p := tea.NewProgram(m, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}
