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

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func RandStringBytes(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

func generateJWT() (string, error) {
	time.Sleep(time.Millisecond)
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

// getTarget is an implementation specific, thread safe function that will create a call and return the target format required by the test
func getTarget() (vegeta.Target, error) {
	targetter := vegeta.Target{}

	targetter.URL = "https://rc-billing.smokeball.com/v2/billing/staff-permissions/authorise-user/42a1f054-820c-42c6-bc77-ed6b2ce67fac/"

	targetter.Method = "GET"
	header := http.Header{}

	newToken, err := generateJWT()

	if err != nil {
		return targetter, err
	}

	header.Set("Authorization", "Bearer "+newToken)

	targetter.Header = header

	return targetter, nil
}

// getTargets is a generic target fetcher
func getTargets(numberOfTargets int) []vegeta.Target {
	fmt.Println("Creating targets")

	defer timeTrack(time.Now(), "Target creation")

	var targets []vegeta.Target

	for i := 1; i <= numberOfTargets; i++ {
		target, err := getTarget()
		if err != nil {
			fmt.Println(err.Error())
			continue
		}
		targets = append(targets, target)
	}
	return targets
}

func orchestrateAttack(vectors []Vector) {
	for _, vector := range vectors {
		rate := vegeta.Rate{Freq: vector.rate, Per: time.Second}

		// Preloads all our data since each request is unique. Otherwise we wouldn't be able to sustain the require rate a load test requires
		targets := getTargets(vector.numberOfTargets)

		// Loads the targets into a round robin targetter (otherwise when requests > targets then we will receive an error)
		targeter := vegeta.NewStaticTargeter(targets...)
		attacker := vegeta.NewAttacker()

		var metrics vegeta.Metrics
		var (
			rep    vegeta.Reporter
			report vegeta.Report
		)

		rep, report = vegeta.NewTextReporter(&metrics), &metrics

		fmt.Println("started attack")
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
}

func main() {
	attackSession := []Vector{
		// {rate: 1, duration: time.Second, numberOfTargets: 10000},
		// {rate: 10, duration: 10 * time.Second, numberOfTargets: 10000},
		// {rate: 100, duration: time.Second, numberOfTargets: 10000},
		{rate: 100, duration: 5 * time.Second, numberOfTargets: 10000},
		// {rate: 250, duration: time.Second, numberOfTargets: 10000},
		// {rate: 250, duration: 5 * time.Second, numberOfTargets: 10000},
	}
	orchestrateAttack(attackSession)
}
