package main

import (
	"encoding/json"
	"github.com/mailgun/graphite-golang"
	//"github.com/mitchellh/mapstructure"
	"github.com/op/go-logging"
	"github.com/streadway/amqp"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
)

var log = logging.MustGetLogger("rabbit-graphite")
var format = logging.MustStringFormatter(
	`%{color}%{time:15:04:05.000} %{shortfunc} ▶ %{level:.4s} %{id:03x}%{color:reset} %{message}`,
)

// Config represents the main YAML configuration
type Config struct {
	Rabbit struct {
		ConnectionString string   `yaml:"connectionString"`
		ConsumerTag      string   `yaml:"consumerTag"`
		Queues           []string `yaml:"queues"`
		Ack              bool     `yaml:"ack"`
	}

	HTTP struct {
		Address string `yaml:"address"`
	}

	Graphite struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"`
	}
}

var (
	config Config
	waiter chan (bool)
	stats  map[string]int64
)

type metricData struct {
	name      string
	value     string
	timestamp int64
}

// loads config.yml into the global config object
func loadConfig() {
	buffer, err := ioutil.ReadFile("config.yml")
	if err != nil {
		log.Error("error: could not open config.yml: %s", err)
	}

	err = yaml.Unmarshal(buffer, &config)
	if err != nil {
		log.Critical("error: could not deserialize config.yml: %s", err)
	}
}

// http status endpoint
func statusHandler(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(stats)
}

func main() {
	log.Info("Rabbit-Carbon Passthrough starting")
	loadConfig()

	conn, err := amqp.Dial(config.Rabbit.ConnectionString)
	if err != nil {
		log.Critical("error: could not connect: %s", err)
	}

	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Critical("error: could not open channel: %s\n", err)
	}

	g, err := graphite.NewGraphite(config.Graphite.Host, config.Graphite.Port)
	if err != nil {
		log.Error("Could not connect, exiting")
		os.Exit(1)
	}

	stats = make(map[string]int64)
	stats["ok"] = 0
	stats["errors"] = 0

	// start HTTP endpoint
	http.HandleFunc("/status", statusHandler)
	go func() {
		log.Infof("HTTP endpoint accepting requests on http://%s", config.HTTP.Address)
		if err := http.ListenAndServe(config.HTTP.Address, nil); err != nil {
			log.Infof("warning: could not start HTTP endpoint (specified address: %s)", config.HTTP.Address)
		}
	}()

	// start a coroutine for each queue
	for _, queue := range config.Rabbit.Queues {
		msgs, err := ch.Consume(queue, config.Rabbit.ConsumerTag, false, false, false, false, nil)
		if err != nil {
			log.Critical("error: could not subscribe: ", err)
		}

		log.Infof("Bound to queue %s", queue)

		go func() {
			for m := range msgs {
				//log.Infof("Received message: %s", m.Body)
				s := string(m.Body[:])
				r := strings.Fields(s)

				// ugly conversion due to graphite wanting a struct with certain data
				data := r[0]
				value := r[1]

				timestamp, err := strconv.ParseInt(r[2], 10, 64)
				if err != nil {
					log.Errorf("Could not convert %s", err)
				}

				if len(r) > 3 {
					log.Infof("longer %s", len(r))
				}

				// make a struct for graphite sender
				message := graphite.Metric{data, value, timestamp}

				if err := g.SendMetric(message); err != nil {
					stats["errors"]++
				} else {
					if config.Rabbit.Ack {
						m.Ack(false) // dont ack since it removes the entry from the queue
					}
					stats["ok"]++
				}

				if stats["errors"] > 10 {
					os.Exit(1)
				}
			}
		}()
	}

	<-waiter
}
