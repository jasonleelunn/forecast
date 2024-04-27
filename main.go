package main

import (
	"encoding/json"
	"fmt"
	"log"
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
	forecastData       forecastData
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

type forecastData struct {
	Time          string
	WeatherCode   string
	UV            string
	WindDirection string
	WindSpeed     string
	Visibility    string
	Precipitation string
	Humidity      string
	GustSpeed     string
	Temperature   string
	FeelsLikeTemp string
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

type color int

const (
	baseUrl = "http://datapoint.metoffice.gov.uk/public/data/"

	dailyResolution       resolution = "daily"
	threeHourlyResolution resolution = "3hourly"
)

const (
	black color = iota
	white
	grey
	green
	blue
	yellow
	pink
	purple
)

var (
	apiKey = os.Getenv("MET_OFFICE_API_KEY")

	colorPalette = map[color]string{
		black:  "#000",
		white:  "#ffffff",
		grey:   "#dddddf",
		green:  "#98FF98",
		blue:   "#a9def9",
		yellow: "#fcf6bd",
		pink:   "#ff99c8",
		purple: "#e4c1f9",
	}

	borderStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color(colorPalette[blue]))

	listStyle = lipgloss.NewStyle().Margin(1, 2)

	tableStyle         table.Styles
	tableStyleFocussed table.Styles

	placenames []string
	rows       Rows
)

// flatten Forecast JSON object returned by API into a consistent format
func getForecastData(m model, f data.Forecast) forecastData {
	if m.forecastResolution == dailyResolution && f.Time == "Day" {
		return forecastData{
			Time:          f.Time,
			WeatherCode:   f.WeatherCode,
			WindDirection: f.WindDirection,
			WindSpeed:     f.WindSpeed,
			Visibility:    f.Visibility,
			UV:            f.Day.UV,
			Precipitation: f.Day.Precipitation,
			Humidity:      f.Day.Humidity,
			GustSpeed:     f.Day.GustSpeed,
			Temperature:   f.Day.Temperature,
			FeelsLikeTemp: f.Day.FeelsLikeTemp,
		}
	} else if m.forecastResolution == dailyResolution && f.Time == "Night" {
		return forecastData{
			Time:          f.Time,
			WeatherCode:   f.WeatherCode,
			WindDirection: f.WindDirection,
			WindSpeed:     f.WindSpeed,
			Visibility:    f.Visibility,
			Precipitation: f.Night.Precipitation,
			Humidity:      f.Night.Humidity,
			GustSpeed:     f.Night.GustSpeed,
			Temperature:   f.Night.Temperature,
			FeelsLikeTemp: f.Night.FeelsLikeTemp,
		}
	} else {
		return forecastData{
			Time:          f.Time,
			WeatherCode:   f.WeatherCode,
			WindDirection: f.WindDirection,
			WindSpeed:     f.WindSpeed,
			Visibility:    f.Visibility,
			UV:            f.Hourly.UV,
			Precipitation: f.Hourly.Precipitation,
			Humidity:      f.Hourly.Humidity,
			GustSpeed:     f.Hourly.GustSpeed,
			Temperature:   f.Hourly.Temperature,
			FeelsLikeTemp: f.Hourly.FeelsLikeTemp,
		}
	}
}

func makeUrl(endpoint string, paramList ...string) string {
	params := ""
	for _, param := range paramList {
		params += "&" + param
	}

	return baseUrl + endpoint + "?key=" + apiKey + params
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

	headerStyle := lipgloss.NewStyle().
		Padding(0, 1).
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color(colorPalette[blue])).
		BorderBottom(true).
		Bold(false)

	tableStyle = table.DefaultStyles()
	tableStyle.Header = headerStyle
	tableStyle.Selected = lipgloss.NewStyle()

	tableStyleFocussed = table.DefaultStyles()
	tableStyleFocussed.Header = headerStyle
	tableStyleFocussed.Selected = tableStyleFocussed.Selected.
		Foreground(lipgloss.Color(colorPalette[black])).
		Background(lipgloss.Color(colorPalette[green])).
		Bold(false)

	// table is out of focus on load
	t.SetStyles(tableStyle)

	return t
}

func setupTextInput() textinput.Model {
	ti := textinput.New()
	ti.Placeholder = "Search for a placename"
	ti.Focus()
	ti.CharLimit = 156

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
	res := data.Fetch(url)
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
	res := data.Fetch(url)
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
		date, err := time.Parse("2006-01-02Z", period.Date)
		if err != nil {
			log.Fatal("Failed to parse date", err)
		}

		for fIndex, forecast := range period.Forecasts {
			forecastData := getForecastData(m, forecast)

			code := forecastData.WeatherCode
			desc := data.WeatherCodes[code]
			desc += " | " + forecastData.Temperature + "°C"
			desc += " | " + forecastData.WindSpeed + "mph"

			var forecastTime = forecastData.Time

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
				m.table.SetStyles(tableStyleFocussed)
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
				m.table.SetStyles(tableStyle)
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
			forecast := m.siteData.Site.Info.Location.Periods[periodIndex].Forecasts[forecastIndex]

			m.forecastData = getForecastData(m, forecast)
		case "r":
			// switch forecast list resolution
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

	return s
}

func searchView(m model) string {
	renderedTable := borderStyle.Render(m.table.View())

	// set the text input width to match the table
	// the text input width is not the full rendered width,
	// just the number of chars in the input field type so
	// we need to adjust by the width of the prompt and border area
	textInputPadding := 5
	m.textInput.Width = lipgloss.Width(renderedTable) - textInputPadding

	components := borderStyle.Render(m.textInput.View()) + "\n" + renderedTable

	// horizontally center the entire view
	gap := strings.Repeat(" ", max(0, (m.width-lipgloss.Width(components))/2))
	return lipgloss.JoinHorizontal(lipgloss.Left, gap, components)
}

func locationView(m model) string {
	return listStyle.Render(m.list.View())
}

func forecastView(m model) string {
	period := m.list.SelectedItem().(forecastItem).Title()
	title := m.siteData.Site.Info.Location.Name + " - " + period

	// TODO: prettier rendering
	forecast := data.WeatherCodes[m.forecastData.WeatherCode] + "\n" +
		m.forecastData.Precipitation + "% chance of rain" + "\n" +
		m.forecastData.Temperature + "°C" + "\n" +
		m.forecastData.WindSpeed + "mph Wind" + "\n" +
		m.forecastData.WindDirection + " Wind Direction" + "\n" +
		m.forecastData.Humidity + "% Humidity" + "\n"

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
