package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"time"

	vegeta "github.com/tsenart/vegeta/lib"
)

type Vector struct {
	rate            int
	duration        time.Duration
	numberOfTargets int
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}

func retry(err error, chann chan []byte, env string) {
	fmt.Println("Error, sleeping to try again another day")
	fmt.Println(err.Error())
	time.Sleep(time.Second)
	getTarget(chann, env)
}

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func RandStringBytes(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

// getTarget is an implementation specific, thread safe function that will create a customer and return the target format required by the attacker
func getTarget(chann chan []byte, env string) {
	// The target API is urlencoded, not json
	jsonTargetter := make(map[string]string)

	cacheBuster := string(rand.Intn(100000))
	searchTerm := "kei"
	// searchTerm := RandStringBytes(rand.Intn(5) + 3)

	jsonTargetter["url"] = "https://perf.justconnect.justice.nsw.gov.au/api/users/lookup/" + searchTerm + "?_=" + cacheBuster

	jsonTargetter["method"] = "GET"

	targetter, err := json.Marshal(jsonTargetter)
	if err != nil {
		retry(err, chann, env)
		return
	}

	chann <- targetter
}

// getTargets is a generic target fetcher
func getTargets(numberOfTargets int, env string) []byte {
	// Hitting the server with 1000 concurrent requests is a good way to have 0 successful responses
	var targets []byte
	// Create a message channel for thread communication that can hold all our uses
	messages := make(chan []byte, numberOfTargets)

	fmt.Println("Creating targets")

	for i := 1; i <= numberOfTargets; i++ {
		// Create a new goroutine (like a thread, but lightweight - more like an Erlang process)
		go getTarget(messages, env)

		// Pulling from a channel will block until a result is available.
		msg := <-messages
		targets = append(targets, msg...)
		targets = append(targets, []byte("\n")...)
		fmt.Printf("Completed %v of %v\n", i, numberOfTargets)
	}

	fmt.Println("Done")
	return targets
}

func orchestrateAttack(vectors []Vector, env string) {
	for _, vector := range vectors {
		rate := vegeta.Rate{Freq: vector.rate, Per: time.Second}

		header := http.Header{}
		header.Set("Authorization", "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySWQiOiI1Yjk4NzFjY2E5MzJmYjAwMGY3MDljYzIiLCJ1c2VyVHlwZSI6IlNVUEVSX1VTRVIiLCJ1c2VyQWdlbmN5IjoiNTlkNmI5NjI3YmNiM2U0NmZjZDk2N2Y5IiwidXNlckFnZW5jeU5hbWUiOiJKdXN0IENvbm5lY3QiLCJ1c2VyQWdlbmN5S2V5IjoiSkMiLCJ1c2VyTG9jYXRpb25zIjpbIjU5ZDZiOTYyN2JjYjNlNDZmY2Q5NjgwNiJdLCJ1c2VyUm9sZXMiOlt7Im5hbWUiOiJKdXN0IENvbm5lY3QgUHJvZHVjdCBPd25lciIsInBlcm1pc3Npb25zIjpbIio6KjoqOioiLCI1OWQ2Yjk2MjdiY2IzZTQ2ZmNkOTY3Zjk6Kjp1c2VyOnJlYWQiXSwidGFncyI6WyJDQU5fQ0FOQ0VMX1BFTkRJTkdfQVBQT0lOVE1FTlQiXSwiY3JlYXRlZERhdGUiOiIyMDE3LTEwLTA1VDIyOjU5OjUwLjA5MVoiLCJsYXN0TW9kaWZpZWREYXRlIjoiMjAxOC0wNi0wMVQwNjo1MToyOC4xOTVaIiwiaWQiOiI1OWQ2Yjk2NjdiY2IzZTQ2ZmNkOTZiZmIiLCJhZ2VuY3lJZCI6IjU5ZDZiOTYyN2JjYjNlNDZmY2Q5NjdmOSIsImxhc3RNb2RpZmllZEJ5SWQiOiI1YTI5YjEzNjA0YTU1YjAwMGY4MDdjOWMiLCJrZXkiOiJTVVBFUl9BRE1JTiJ9XSwidXNlclRhZ3MiOlsiQ0FOX0NBTkNFTF9QRU5ESU5HX0FQUE9JTlRNRU5UIl0sInByZXZpb3VzTG9naW5EYXRlIjoiMjAxOC0wOS0xNFQwNDo0MTozOS4yMTVaIiwiY3VycmVudExvZ2luRGF0ZSI6IjIwMTgtMTAtMDVUMDI6Mjg6MDMuMjY5WiIsImV4cCI6MTUzODcxMDA4MywiaWF0IjoxNTM4NzA2NDgzfQ.zaKqUzLlwhzr9e_mMFW7U8XRJLzvgh12VCvmM0LH2Ng")
		src := getTargets(vector.numberOfTargets, env)

		// Preloads all our attack data since each request is unique. Otherwise we wouldn't be able to sustain the require rate a loadtest requires
		targets, err := vegeta.ReadAllTargets(vegeta.NewJSONTargeter(bytes.NewReader(src), []byte(""), header))
		check(err)

		// Loads the targets into a round robin targetter (otherwise when requests > targets then we will receive an error)
		targeter := vegeta.NewStaticTargeter(targets...)
		attacker := vegeta.NewAttacker()

		var metrics vegeta.Metrics
		var (
			rep    vegeta.Reporter
			report vegeta.Report
		)

		rep, report = vegeta.NewTextReporter(&metrics), &metrics

		fmt.Println("Started vector")
		for res := range attacker.Attack(targeter, rate, vector.duration, "perf") {
			//print body
			// fmt.Println(string(res.Body))
			report.Add(res)
		}
		if c, ok := report.(vegeta.Closer); ok {
			c.Close()
		}

		fmt.Printf("\nVector:\nRate: %v, Duration: %v, Unique Targets: %v\n", rate, vector.duration, vector.numberOfTargets)
		metrics.Errors = []string{}
		rep.Report(os.Stdout)
	}
	fmt.Println("Finished attack")
}

func main() {
	env := "local"
	attackSession := []Vector{
		// Vector{rate: 10, duration: time.Second, numberOfTargets: 10},
		Vector{rate: 30, duration: 10 * time.Second, numberOfTargets: 1000},
		// Vector{rate: 100, duration: time.Second, numberOfTargets: 10},
		// Vector{rate: 100, duration: 5 * time.Second, numberOfTargets: 10},
		// Vector{rate: 250, duration: time.Second, numberOfTargets: 10},
		// Vector{rate: 250, duration: 5 * time.Second, numberOfTargets: 10},
	}
	orchestrateAttack(attackSession, env)
}
