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

// getTarget is an implementation specific, thread safe function that will create a customer and return the target format required by the attacker
func getTarget() []byte {
	jsonDetails := make(map[string]string)
	jsonDetails["product"] = "account"
	jsonDetails["env"] = "sandbox"
	jsonDetails["token"] = "AutomationTest"
	body, err := json.Marshal(jsonDetails)
	if err != nil {
		fmt.Println("Error, sleeping to try again another day")
		fmt.Println(err.Error())
		time.Sleep(time.Second)
		return getTarget()
	}
	rsp, err := http.Post("http://localhost:3001/createuser", "application/json", bytes.NewReader(body))
	if err != nil {
		fmt.Println("Error, sleeping to try again another day")
		fmt.Println(err.Error())
		time.Sleep(time.Second)
		return getTarget()
	}
	body_byte, err := ioutil.ReadAll(rsp.Body)
	if err != nil {
		fmt.Println("Error, sleeping to try again another day")
		fmt.Println(err.Error())
		time.Sleep(time.Second)
		return getTarget()
	}

	data := url.Values{}
	data.Set("username", string(body_byte))
	data.Set("password", "test1234")
	data.Set("client_id", "zip.ios.wallet")
	data.Set("client_secret", "secret")
	data.Set("grant_type", "password")

	jsonTargetter := make(map[string]string)
	jsonTargetter["method"] = "POST"
	jsonTargetter["body"] = data.Encode()
	jsonTargetter["url"] = "http://dev1.zip.co/login/connect/token"

	body, err = json.Marshal(jsonTargetter)
	if err != nil {
		fmt.Println("Error, sleeping to try again another day")
		fmt.Println(err.Error())
		time.Sleep(time.Second)
		return getTarget()
	}
	var encodedBody []byte
	base64.StdEncoding.Encode(encodedBody, body)

	return encodedBody
}

// getTargets is a generic target fetcher
func getTargets(numberOfTargets int) []byte {
	var targets []byte
	fmt.Println("Creating users")

	for i := 1; i <= numberOfTargets; i++ {
		targets = append(targets, getTarget()...)
		fmt.Printf("Completed %v of %v\n", i, numberOfTargets)
	}
	fmt.Println("Done")
	return targets
}

func orchestrateAttack(vectors []Vector) {
	for _, vector := range vectors {
		rate := uint64(vector.rate) // per second
		duration := vector.duration

		header := http.Header{}
		header.Set("content-type", "application/x-www-form-urlencoded")
		src := getTargets(vector.numberOfTargets)
		targeter := vegeta.NewJSONTargeter(bytes.NewReader(src), []byte(""), header)

		attacker := vegeta.NewAttacker()

		var metrics vegeta.Metrics
		fmt.Println("started attack")
		for res := range attacker.Attack(targeter, rate, duration, "perf") {
			metrics.Add(res)
		}
		metrics.Close()

		fmt.Printf("%+v\n", metrics)
	}
}

func main() {
	attackSession := []Vector{
		Vector{rate: 10, duration: time.Second, numberOfTargets: 10},
		Vector{rate: 10, duration: 10 * time.Second, numberOfTargets: 100},
		Vector{rate: 100, duration: time.Second, numberOfTargets: 100},
		Vector{rate: 100, duration: 5 * time.Second, numberOfTargets: 100},
		Vector{rate: 250, duration: time.Second, numberOfTargets: 100},
		Vector{rate: 250, duration: 5 * time.Second, numberOfTargets: 100},
	}
	orchestrateAttack(attackSession)
}
