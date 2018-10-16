package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	calendar "google.golang.org/api/calendar/v3"
)

// Retrieve a token, saves the token, then returns the generated client.
func getClient(config *oauth2.Config) *http.Client {
	tokFile := os.ExpandEnv("${HOME}/.config/gcal-conky/token.json")
	tok, err := tokenFromFile(tokFile)
	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken(tokFile, tok)
	}
	return config.Client(context.Background(), tok)
}

// Request a token from the web, then returns the retrieved token.
func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		log.Fatalf("Unable to read authorization code: %v", err)
	}

	tok, err := config.Exchange(oauth2.NoContext, authCode)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web: %v", err)
	}
	return tok
}

// Retrieves a token from a local file.
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	defer f.Close()
	if err != nil {
		return nil, err
	}
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// Saves a token to a file path.
func saveToken(path string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	defer f.Close()
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	json.NewEncoder(f).Encode(token)
}

func color(c int, str string) string {
	return fmt.Sprintf("${color%v}%v${color0}", c, str)
}

func isToday(t time.Time) bool {
	today := time.Now()
	return t.Day() == today.Day() && t.Month() == today.Month() && t.Year() == today.Year()
}

func isTomorrow(t time.Time) bool {
	return isToday(t.AddDate(0, 0, -1))
}

func printCal() []string {
	weeks := 14
	output := make([]string, weeks+1)

	today := time.Now()
	firstDayOfCurrentWeek := firstDayOfWeek(today)

	output[0] = color(1, fmt.Sprint("    Mo Di Mi Do Fr Sa So "))
	for week := 0; week < weeks; week++ {
		for weekday := 0; weekday < 7; weekday++ {
			day := firstDayOfCurrentWeek.AddDate(0, 0, week*7+weekday)

			if day == firstDayOfWeek(day) {
				if lastDayOfWeek := lastDayOfWeek(day); lastDayOfWeek.Day() <= 7 {
					output[week+1] = fmt.Sprintf("%v ", color(1, lastDayOfWeek.Month().String()[0:3]))
				} else {
					output[week+1] = fmt.Sprint("    ")
				}
			}

			if isToday(day) {
				output[week+1] += color(1, fmt.Sprintf("%2d ", day.Day()))
			} else {
				output[week+1] += fmt.Sprintf("%2d ", day.Day())
			}
		}
	}
	return output
}

func firstDayOfWeek(t time.Time) time.Time {
	return t.AddDate(0, 0, 1-int(t.Weekday()))
}

func lastDayOfWeek(t time.Time) time.Time {
	return firstDayOfWeek(t).AddDate(0, 0, 6)
}

func entries() []string {
	b, err := ioutil.ReadFile(os.ExpandEnv("${HOME}/.config/gcal-conky/credentials.json"))
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}

	// If modifying these scopes, delete your previously saved token.json.
	config, err := google.ConfigFromJSON(b, calendar.CalendarReadonlyScope)
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	client := getClient(config)

	srv, err := calendar.New(client)
	if err != nil {
		log.Fatalf("Unable to retrieve Calendar client: %v", err)
	}

	t := time.Now().Format(time.RFC3339)
	events, err := srv.Events.List("primary").ShowDeleted(false).
		SingleEvents(true).TimeMin(t).MaxResults(10).OrderBy("startTime").Do()
	if err != nil {
		log.Fatalf("Unable to retrieve next ten of the user's events: %v", err)
	}

	if len(events.Items) == 0 {
		return []string{"No upcoming events found."}
	}

	output := make([]string, 0, 20)
	var lastDate string

	for _, item := range events.Items {
		date := item.Start.DateTime
		if date == "" {
			date = item.Start.Date
		}
		startTime := item.Start.DateTime[11:16]
		endTime := item.End.DateTime[11:16]

		if currentDate := date[0:10]; lastDate != currentDate {
			parsedDate, _ := time.Parse("2006-01-02", currentDate)

			if isToday(parsedDate) {
				output = append(output, fmt.Sprintf("Today, %v", currentDate))
			} else if isTomorrow(parsedDate) {
				output = append(output, fmt.Sprintf("Tomorrow, %v", currentDate))
			} else {
				output = append(output, fmt.Sprintf("%v, %v", parsedDate.Weekday(), currentDate))
			}
			lastDate = currentDate
		}

		// TODO allDay events
		timeStr := fmt.Sprintf("%v-%v", startTime, endTime)
		output = append(output, fmt.Sprintf("%v [%v] %v", color(1, timeStr), item.Status[0:3], item.Summary))
		if len(item.Location) > 0 {
			output = append(output, fmt.Sprintf("            @%v", item.Location))
		}
	}
	return output
}

func max(a int, b int) int {
	if a > b {
		return a
	}
	return b
}

func main() {
	cal := printCal()
	entries := entries()

	for i := 0; i < max(len(cal), len(entries)); i++ {
		if i < len(cal) {
			fmt.Print(cal[i])
			fmt.Print("    ")
		} else {
			fmt.Print("                             ")
		}

		if i < len(entries) {
			fmt.Println(strings.Replace(entries[i], "#", "\\#", -1))
		} else {
			fmt.Println("")
		}
	}
}
