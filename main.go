package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	vegeta "github.com/tsenart/vegeta/lib"
)

// I HAVE PATCHED THE VEGETA LIBRARY TO ACCEPT URL ENCODED POSTS
// DO NOT UPDATE THE VENDOR LIBS

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

// getTarget is an implementation specific, thread safe function that will create a customer and return the target format required by the attacker
func getTarget(chann chan []byte, env string) {
	// The target API is urlencoded, not json
	data := url.Values{}
	jsonTargetter := make(map[string]string)

	// For some reason we need to create the user in the "dev1" domain, but target the "dev" domain
	envString := env
	if env == "dev1" {
		envString = "dev"
	}

	if env != "local" {
		// User API details
		jsonDetails := make(map[string]string)
		jsonDetails["product"] = "account"
		jsonDetails["env"] = env
		jsonDetails["token"] = "AutomationTest"
		body, err := json.Marshal(jsonDetails)
		if err != nil {
			retry(err, chann, env)
			return
		}

		rsp, err := http.Post("http://localhost:3001/createuser", "application/json", bytes.NewReader(body))
		if err != nil {
			retry(err, chann, env)
			return
		}

		body_byte, err := ioutil.ReadAll(rsp.Body)
		if err != nil {
			retry(err, chann, env)
			return
		}
		data.Set("username", string(body_byte))
		data.Set("password", "test1234")
		jsonTargetter["url"] = "http://" + envString + ".zip.co/login/connect/token"
	} else {
		data.Set("username", "sweet2@mailinator.com")
		data.Set("password", "123456")
		jsonTargetter["url"] = "http://localhost:5000/login/connect/token"
	}

	data.Set("client_id", "zip.ios.wallet")
	data.Set("client_secret", "secret")
	data.Set("grant_type", "password")

	encodedData := &bytes.Buffer{}
	encoder := base64.NewEncoder(base64.StdEncoding, encodedData)
	defer encoder.Close()
	encoder.Write([]byte(data.Encode()))

	jsonTargetter["body"] = encodedData.String()
	jsonTargetter["method"] = "POST"

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
	simultaneousConnections := 5
	var targets []byte
	// Create a message channel for thread communication that can hold all our uses
	messages := make(chan []byte, numberOfTargets)

	fmt.Println("Creating targets")

	for i := 1; i <= numberOfTargets; i++ {
		// Create a new goroutine (like a thread, but lightweight - more like an Erlang process)
		go getTarget(messages, env)

		// Pulling from a channel will block until a result is available.
		if i > simultaneousConnections {
			msg := <-messages
			targets = append(targets, msg...)
			fmt.Printf("Completed %v of %v\n", i-simultaneousConnections, numberOfTargets)
		}
	}

	// Pull the remaining results from the channel
	for i := 1; i <= simultaneousConnections; i++ {
		msg := <-messages
		targets = append(targets, msg...)
		fmt.Printf("Completed %v of %v\n", numberOfTargets-simultaneousConnections+i, numberOfTargets)
	}
	fmt.Println("Done")
	return targets
}

func orchestrateAttack(vectors []Vector, env string) {
	for _, vector := range vectors {
		rate := uint64(vector.rate) // per second

		header := http.Header{}
		header.Set("content-type", "application/x-www-form-urlencoded")
		src := getTargets(vector.numberOfTargets, env)

		// Preloads all our attack data since each request is unique. Otherwise we wouldn't be able to sustain the require rate a loadtest requires
		targets, err := vegeta.ReadAllTargets(vegeta.NewJSONTargeter(bytes.NewReader(src), []byte(""), header))
		check(err)

		// Loads the targets into a round robin targetter (otherwise when requests > targets then we will receive an error)
		targeter := vegeta.NewStaticTargeter(targets...)
		attacker := vegeta.NewAttacker()

		var metrics vegeta.Metrics

		fmt.Println("Started vector")
		for res := range attacker.Attack(targeter, rate, vector.duration, "perf") {
			metrics.Add(res)
		}
		metrics.Close()

		fmt.Printf("\nVector:\nRate: %v, Duration: %v, Unique Targets: %v\n", rate, vector.duration, vector.numberOfTargets)
		fmt.Printf("%+v\n", metrics)
	}
	fmt.Println("Finished attack")
}

func main() {
	env := "local"
	attackSession := []Vector{
		// Vector{rate: 10, duration: time.Second, numberOfTargets: 10},
		Vector{rate: 10, duration: 10 * time.Second, numberOfTargets: 10},
		// Vector{rate: 100, duration: time.Second, numberOfTargets: 10},
		// Vector{rate: 100, duration: 5 * time.Second, numberOfTargets: 10},
		// Vector{rate: 250, duration: time.Second, numberOfTargets: 10},
		// Vector{rate: 250, duration: 5 * time.Second, numberOfTargets: 10},
	}
	orchestrateAttack(attackSession, env)
}
