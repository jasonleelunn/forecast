# forecast

U.K. weather forecasts, in your terminal :sunny: :cloud_with_lightning_and_rain:

![forecast GIF](./forecast.gif)

## Running locally

- Register for a Met Office DataPoint API key [here](https://register.metoffice.gov.uk/WaveRegistrationClient/public/register.do?service=datapoint)
- Add the API key to your env

```sh
export MET_OFFICE_API_KEY=<your_key_here>
```

- Clone this repository and navigate to it

```sh
git clone git@github.com:jasonleelunn/forecast.git
cd forecast
```

- Build the executable (requires Go 1.21)

```sh
go build
```

- Run the application

```sh
./forecast
```

## Usage

- Press Enter to move to the next view
- Press Esc to move to the previous view
- Press Ctrl+c to exit
