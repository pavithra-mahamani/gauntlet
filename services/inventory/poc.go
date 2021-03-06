// main.go
package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	gocb "github.com/couchbase/gocb/v2"
	"github.com/gorilla/mux"
)

type Seat struct {
	SeatNumber string `json:"seatnumber"`
	Available  bool   `json:"available"`
	Class      string `json:"class"`
	Price      int64  `json:"price"`
}

// Flight
type Flight struct {
	FlightID         string `json:"flight_id"`
	Status           string `json:"status"`
	Airline          string `json:"airline"`
	Model            string `json:"model"`
	DepartureDate    string `json:"departure_date"`
	DepartingAirport string `json:"departing_airport"`
	ArrivingAirport  string `json:"arriving_airport"`
	Seats            []Seat `json:"seats"`
}

var CBCluster *gocb.Cluster
var E2EBucket *gocb.Bucket
var InventoryScope *gocb.Scope
var FlightsCollection *gocb.Collection
var err error

func createNewFlight(w http.ResponseWriter, r *http.Request) {
	reqBody, _ := ioutil.ReadAll(r.Body)
	defer r.Body.Close()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	var newflight Flight
	json.Unmarshal(reqBody, &newflight)

    log.Println("Creating flight %s", newflight.FlightID)
	_, err = FlightsCollection.Upsert(newflight.FlightID, newflight, nil)
	if err != nil {
		log.Fatal(err)
	}
	// Get the document back
	getResult, err := FlightsCollection.Get(newflight.FlightID, nil)
	if err != nil {
		log.Fatal(err)
	}

	var f interface{}
	if err := getResult.Content(&f); err != nil {
		panic(err)
	}
	json.NewEncoder(w).Encode(f)
}
func getFlightById(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Endpoint Hit: flight")
	vars := mux.Vars(r)
	key := vars["flight_id"]

	getResult, err := FlightsCollection.Get(key, &gocb.GetOptions{})
	if err != nil {
		panic(err)
	}
	var myFlight interface{}
	if err := getResult.Content(&myFlight); err != nil {
		panic(err)
	}
	fmt.Println(myFlight)
	json.NewEncoder(w).Encode(myFlight)
}
func getAllFlights(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Endpoint Hit: flights")
	vars := mux.Vars(r)
	airlineParam := vars["airline"]
	fmt.Println(airlineParam)
	// Perform a N1QL Query
	query := fmt.Sprintf("SELECT * FROM `e2e`.inventory.flights WHERE airline='%s' LIMIT 10;", airlineParam)
	fmt.Println(query)
	queryResult, err := CBCluster.Query(query, &gocb.QueryOptions{})
	// check query was successful
	if err != nil {
		panic(err)
	}
	var Flights []interface{}
	// Print each found Row
	for queryResult.Next() {
		var result interface{}
		err := queryResult.Row(&result)
		if err != nil {
			panic(err)
		}
		fmt.Println(result)
		Flights = append(Flights, result)
	}

	if err := queryResult.Err(); err != nil {
		panic(err)
	}
	json.NewEncoder(w).Encode(Flights)
}
func handleRequests() {
	myRouter := mux.NewRouter().StrictSlash(true)
	myRouter.HandleFunc("/flights/{airline}", getAllFlights)
	myRouter.HandleFunc("/newflight", createNewFlight).Methods("POST")
	myRouter.HandleFunc("/flight/{flight_id}", getFlightById)

	log.Fatal(http.ListenAndServe(":10000", myRouter))
}

func main() {
	hostname := os.Getenv("DB_HOSTNAME")
	bucketName := "e2e"
	scope_name := "inventory"
	collection_name := "flights"
	username := os.Getenv("CAPELLA_USERNAME")
	password := os.Getenv("CAPELLA_PASSWORD")

	// Initialize the connection
	cluster, err := gocb.Connect("couchbases://"+hostname+"?ssl=no_verify", gocb.ClusterOptions{
		Authenticator: gocb.PasswordAuthenticator{
			Username: username,
			Password: password,
		},
	})
	if err != nil {
		log.Fatal(err)
	}

    CBCluster = cluster
	bucket := cluster.Bucket(bucketName)
	err = bucket.WaitUntilReady(60*time.Second, nil)
	if err != nil {
		log.Fatal(err)
	}
	// Get a user-defined collection reference
	InventoryScope = bucket.Scope(scope_name)
	FlightsCollection = InventoryScope.Collection(collection_name)

    queryResult, err := CBCluster.Query("CREATE PRIMARY INDEX inventory_flights_pri_index ON e2e.inventory.flights", &gocb.QueryOptions{})
    for queryResult.Next() {
		var result interface{}
		err := queryResult.Row(&result)
		if err != nil {
			panic(err)
		}
		fmt.Println(result)
	}
    if err = queryResult.Err(); err != nil {
		panic(err)
	}
// 	cluster.QueryIndexes().CreatePrimaryIndex(bucketName, &gocb.CreatePrimaryQueryIndexOptions{
// 		IgnoreIfExists: true,
// 		CustomName: "flight_inventory_primary_index",
// 		ScopeName: scope_name,
// 		CollectionName: collection_name,
// 	})

	handleRequests()
}
