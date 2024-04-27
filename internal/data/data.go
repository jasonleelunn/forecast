package data

import (
	"fmt"
	"io"
	"net/http"
	"time"
)

// see https://www.metoffice.gov.uk/binaries/content/assets/metofficegovuk/pdf/data/datapoint_api_reference.pdf
// for full API schema details

// some codes are duplicated for (day) and (night)
var WeatherCodes = map[string]string{
	"0":  "Clear night",
	"1":  "Sunny day",
	"2":  "Partly cloudy",
	"3":  "Partly cloudy",
	"4":  "Not used",
	"5":  "Mist",
	"6":  "Fog",
	"7":  "Cloudy",
	"8":  "Overcast",
	"9":  "Light rain shower",
	"10": "Light rain shower",
	"11": "Drizzle",
	"12": "Light rain",
	"13": "Heavy rain shower",
	"14": "Heavy rain shower",
	"15": "Heavy rain",
	"16": "Sleet shower",
	"17": "Sleet shower",
	"18": "Sleet",
	"19": "Hail shower",
	"20": "Hail shower",
	"21": "Hail",
	"22": "Light snow shower",
	"23": "Light snow shower",
	"24": "Light snow",
	"25": "Heavy snow shower",
	"26": "Heavy snow shower",
	"27": "Heavy snow",
	"28": "Thunder shower",
	"29": "Thunder shower",
	"30": "Thunder",
}

type Forecast struct {
	Time          string `json:"$"`
	WeatherCode   string `json:"W"`
	Visibility    string `json:"V"`
	WindDirection string `json:"D"`
	WindSpeed     string `json:"S"`
	Day
	Night
	Hourly
}

type Day struct {
	UV            string `json:"U"`
	Precipitation string `json:"PPd"`
	Humidity      string `json:"Hn"`
	GustSpeed     string `json:"Gn"`
	Temperature   string `json:"Dm"`
	FeelsLikeTemp string `json:"FDm"`
}

type Night struct {
	Precipitation string `json:"PPn"`
	Humidity      string `json:"Hm"`
	GustSpeed     string `json:"Gm"`
	Temperature   string `json:"Nm"`
	FeelsLikeTemp string `json:"FNm"`
}

type Hourly struct {
	UV            string `json:"U"`
	Precipitation string `json:"Pp"`
	Humidity      string `json:"H"`
	GustSpeed     string `json:"G"`
	Temperature   string `json:"T"`
	FeelsLikeTemp string `json:"F"`
}

type Period struct {
	Time      string     `json:"type"`
	Date      string     `json:"value"`
	Forecasts []Forecast `json:"Rep"`
}

type Location struct {
	Id        string   `json:"i"`
	Lat       string   `json:"lat"`
	Lon       string   `json:"lon"`
	Name      string   `json:"name"`
	Country   string   `json:"country"`
	Continent string   `json:"continent"`
	Periods   []Period `json:"Period"`
}

type Info struct {
	Date     string   `json:"dataDate"`
	Length   string   `json:"type"`
	Location Location `json:"Location"`
}

type Param struct {
	Name        string `json:"name"`
	Units       string `json:"units"`
	Description string `json:"$"`
}

type Meta struct {
	Params []Param `json:"Param"`
}

type Site struct {
	MetaInfo Meta `json:"Wx"`
	Info     Info `json:"DV"`
}

type SiteData struct {
	Site Site `json:"SiteRep"`
}

func Fetch(url string) []byte {
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
