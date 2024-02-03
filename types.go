package main

var WeatherCodes = map[string]string{
	"0":  "Clear night",
	"1":  "Sunny day",
	"2":  "Partly cloudy (night)",
	"3":  "Partly cloudy (day)",
	"4":  "Not used",
	"5":  "Mist",
	"6":  "Fog",
	"7":  "Cloudy",
	"8":  "Overcast",
	"9":  "Light rain shower (night)",
	"10": "Light rain shower (day)",
	"11": "Drizzle",
	"12": "Light rain",
	"13": "Heavy rain shower (night)",
	"14": "Heavy rain shower (day)",
	"15": "Heavy rain",
	"16": "Sleet shower (night)",
	"17": "Sleet shower (day)",
	"18": "Sleet",
	"19": "Hail shower (night)",
	"20": "Hail shower (day)",
	"21": "Hail",
	"22": "Light snow shower (night)",
	"23": "Light snow shower (day)",
	"24": "Light snow",
	"25": "Heavy snow shower (night)",
	"26": "Heavy snow shower (day)",
	"27": "Heavy snow",
	"28": "Thunder shower (night)",
	"29": "Thunder shower (day)",
	"30": "Thunder",
}

type Forecast struct {
	UV            string `json:"U"`
	WeatherCode   string `json:"W"`
	Precipitation string `json:"Pp"`
}

type Period struct {
	Length    string     `json:"type"`
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
