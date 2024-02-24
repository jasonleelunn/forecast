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
	"strconv"
	"strings"
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
	width              int
	height             int
	err                error
	textInput          textinput.Model
	table              table.Model
	list               list.Model
	siteData           data.SiteData
	locationChosen     bool
	locationId         string
	forecastResolution resolution
	forecastChosen     bool
	forecastData       data.Forecast
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
	title, desc                string
	periodIndex, forecastIndex int
}

func (i forecastItem) Title() string        { return i.title }
func (i forecastItem) Description() string  { return i.desc }
func (i forecastItem) FilterValue() string  { return i.title }
func (i forecastItem) Position() (int, int) { return i.periodIndex, i.forecastIndex }

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

type resolution string

const (
	baseUrl = "http://datapoint.metoffice.gov.uk/public/data/"

	dailyResolution       resolution = "daily"
	threeHourlyResolution resolution = "3hourly"
)

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
	// remove the default list Quit key bind of 'Esc'
	li.KeyMap.Quit.Unbind()

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
		textInput:          ti,
		table:              t,
		list:               li,
		forecastResolution: dailyResolution,
	}
}

func getSiteData(siteId string, resolution resolution) data.SiteData {
	endpoint := "val/wxfcs/all/json/" + siteId
	param := "res=" + string(resolution)
	url := makeUrl(endpoint, param)
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

func getForecastListItems(m model) []list.Item {
	var forecasts []list.Item

	for pIndex, period := range m.siteData.Site.Info.Location.Periods {
		for fIndex, forecast := range period.Forecasts {
			code := forecast.WeatherCode

			date, err := time.Parse("2006-01-02Z", period.Date)
			if err != nil {
				log.Fatal("Failed to parse date", err)
			}

			var forecastTime = forecast.Time
			var temp string

			// TODO: simplify data response interface to hide day/night/hourly differences
			if forecastTime == "Day" {
				temp = forecast.TemperatureDay
			} else {
				temp = forecast.TemperatureNight
			}

			wind := forecast.WindSpeed

			desc := data.WeatherCodes[code]
			desc += " | " + temp + "°C"
			desc += " | " + wind + "mph"

			if m.forecastResolution == threeHourlyResolution {
				// Time is represented as minutes past midnight here
				// so convert to 24hr clock representation instead
				minutes, err := strconv.Atoi(forecastTime)
				if err != nil {
					log.Fatal("Couldn't convert time", err)
				}

				hours := minutes / 60
				forecastTime = fmt.Sprintf("%02d:00", hours)
			}

			title := date.Format("Mon, 02 Jan 2006") + " (" + forecastTime + ")"

			item := forecastItem{title: title, desc: desc, periodIndex: pIndex, forecastIndex: fIndex}

			forecasts = append(forecasts, item)
		}
	}

	return forecasts
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
		m.width = msg.Width
		m.height = msg.Height

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
				m.locationChosen = true
				m.locationId = m.table.SelectedRow()[1]

				m.siteData = getSiteData(m.locationId, m.forecastResolution)
				forecasts := getForecastListItems(m)
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
	var cmds []tea.Cmd

	var listCmd tea.Cmd
	m.list, listCmd = m.list.Update(msg)
	cmds = append(cmds, listCmd)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			m.forecastChosen = true

			item := m.list.SelectedItem().(forecastItem)
			periodIndex, forecastIndex := item.Position()
			m.forecastData = m.siteData.Site.Info.Location.Periods[periodIndex].Forecasts[forecastIndex]
		case "r":
			if m.forecastResolution == dailyResolution {
				m.forecastResolution = threeHourlyResolution
			} else {
				m.forecastResolution = dailyResolution
			}

			m.siteData = getSiteData(m.locationId, m.forecastResolution)
			forecasts := getForecastListItems(m)
			cmd := m.list.SetItems(forecasts)
			cmds = append(cmds, cmd)
		case "esc":
			m.locationChosen = false
		}
	}

	return m, tea.Batch(cmds...)
}

func updateForecast(msg tea.Msg, m model) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			m.forecastChosen = false
		}
	}

	return m, nil
}

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
	components := baseStyle.Render(m.textInput.View()) + "\n" + baseStyle.Render(m.table.View())

	gap := strings.Repeat(" ", max(0, (m.width-lipgloss.Width(components))/2))
	return lipgloss.JoinHorizontal(lipgloss.Center, gap, components)
}

func locationView(m model) string {
	return listStyle.Render(m.list.View())
}

func forecastView(m model) string {
	period := m.list.SelectedItem().(forecastItem).Title()
	title := m.siteData.Site.Info.Location.Name + " - " + period

	// TODO: handle Night data as well + prettier rendering
	forecast := data.WeatherCodes[m.forecastData.WeatherCode] + "\n" +
		m.forecastData.PrecipitationDay + "% chance of rain" + "\n" +
		m.forecastData.TemperatureDay + "°C" + "\n" +
		m.forecastData.WindSpeed + "mph Wind" + "\n" +
		m.forecastData.WindDirection + " Wind Direction" + "\n" +
		m.forecastData.HumidityDay + "% Humidity" + "\n"

	text := title + "\n\n" + forecast

	return listStyle.Render(text)
}

func main() {
	m := initialModel()
	p := tea.NewProgram(m, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		log.Fatal(err)
	}
}
