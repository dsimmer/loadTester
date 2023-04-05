package main

import (
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/golang-jwt/jwt"

	vegeta "github.com/tsenart/vegeta/lib"
)

func timeTrack(start time.Time, name string) {
	elapsed := time.Since(start)
	log.Printf("%s took %s", name, elapsed)
}

var sampleSecretKey = []byte("JWTSecret")

func generateJWT() (string, error) {
	token := jwt.New(jwt.SigningMethodHS256)
	token.Claims = jwt.MapClaims{
		"exp": time.Now().Add(time.Hour * 72).Unix(),
	}

	tokenString, err := token.SignedString([]byte(RandStringBytes(rand.Intn(5) + 3)))

	if err != nil {
		return "", err
	}

	return tokenString, nil
}

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

func retry(err error, chann chan vegeta.Target, env string) {
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

// getTarget is an implementation specific, thread safe function that will create a call and return the target format required by the test
func getTarget(chann chan vegeta.Target, env string) {
	// The target API is urlencoded, not json
	targetter := vegeta.Target{}

	targetter.URL = "http://localhost:9001/v2/integration/webapp-graphql-server/58379d99-180d-4560-bf1d-ffd5511d0c5f/"
	// targetter.URL = "https://rc-billing.smokeball.com/v2/integration/webapp-graphql-server/58379d99-180d-4560-bf1d-ffd5511d0c5f/"

	targetter.Method = "POST"
	header := http.Header{}

	targetter.Body = []byte(`[{"operationName":"MatterStatusCounts","variables":{"filter":{"excludeBillableLeadMatters":false,"matterStatus":["open","pending","closed","deleted"]}},"query":"query MatterStatusCounts($filter: MatterStatusCountsFilter) {\n  matterStatusCounts(filter: $filter) {\n    status\n    count\n    __typename\n  }\n}\n"},{"operationName":"StaffMembersList","variables":{"filter":{"excludeFormerStaff":true,"excludeStaffWithoutMatter":true}},"query":"query StaffMembersList($filter: StaffMemberFilter) {\n  staffMembersList(filter: $filter) {\n    results {\n      id\n      firstName\n      lastName\n      initials\n      __typename\n    }\n    __typename\n  }\n}\n"},{"operationName":"MatterTypesFilter","variables":{"filter":{"excludeMatterTypeWithoutMatter":true}},"query":"query MatterTypesFilter($filter: MatterTypeFilter) {\n  matterTypeList(filter: $filter) {\n    results {\n      matterTypeId\n      matterTypeIdWithoutLocation\n      name\n      categories\n      __typename\n    }\n    __typename\n  }\n}\n"}]`)
	header.Set("Content-type", "application/json")
	header.Set("cache-control", "no-cache")
	header.Set("Authorization", "Bearer ")

	targetter.Header = header

	chann <- targetter
}

// getTargets is a generic target fetcher
func getTargets(numberOfTargets int, env string) []vegeta.Target {
	fmt.Println("Creating targets")
	defer timeTrack(time.Now(), "Target creation")
	// Hitting the server with 1000 concurrent requests is a good way to have 0 successful responses
	var targets []vegeta.Target
	// Create a message channel for thread communication that can hold all our uses
	messages := make(chan vegeta.Target, numberOfTargets)

	for i := 1; i <= numberOfTargets; i++ {
		// Create a new goroutine (like a thread, but lightweight - more like an Erlang process)
		go getTarget(messages, env)
	}
	for i := 1; i <= numberOfTargets; i++ {
		// Pulling from a channel will block until a result is available.
		msg := <-messages
		targets = append(targets, msg)
		// fmt.Printf("Completed %v of %v\n", i, numberOfTargets)
	}

	return targets
}

func orchestrateAttack(vectors []Vector, env string) {
	for _, vector := range vectors {
		rate := vegeta.Rate{Freq: vector.rate, Per: time.Second}

		// Preloads all our data since each request is unique. Otherwise we wouldn't be able to sustain the require rate a load test requires
		targets := getTargets(vector.numberOfTargets, env)

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
	fmt.Println("Finished load test")
}

func main() {
	env := "local"
	attackSession := []Vector{
		// {rate: 1, duration: time.Second, numberOfTargets: 1},
		// {rate: 10, duration: 10 * time.Second, numberOfTargets: 1},
		// {rate: 100, duration: time.Second, numberOfTargets: 1},
		{rate: 100, duration: 5 * time.Second, numberOfTargets: 1},
		// {rate: 250, duration: time.Second, numberOfTargets: 1},
		// {rate: 250, duration: 5 * time.Second, numberOfTargets: 1},
	}
	orchestrateAttack(attackSession, env)
}
