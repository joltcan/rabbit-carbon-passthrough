package main

import (
	"encoding/json"
	"github.com/caarlos0/env"
	"github.com/mailgun/graphite-golang"
	"github.com/op/go-logging"
	"github.com/streadway/amqp"
	"net/http"
	"os"
	"strconv"
	"strings"
)

var log = logging.MustGetLogger("rabbit-graphite")
var format = logging.MustStringFormatter(
	`%{color}%{time:15:04:05.000} %{shortfunc} ▶ %{level:.4s} %{id:03x}%{color:reset} %{message}`,
)

type Config struct {
	RabbitURI    string   `env:"RABBIT_URI"`
	RabbitTag    string   `env:"RABBIT_TAG"`
	RabbitQueues []string `env:"RABBIT_QUEUES" envSeparator:":"`
	RabbitAck    bool     `env:"RABBIT_ACK" envDefault:false`

	ExitErrors   int64  `env:"EXIT_ERRORS" envDefault:10`
	StatusListen string `env:"STATUS_LISTEN" envDefault:"0.0.0.0:8082"`

	GraphiteHost  string `env:"GRAPHITE_HOST" envDefault:"localhost"`
	GraphitePort  int    `env:"GRAPHITE_PORT" envDefault:"2003"`
	GraphiteWrite bool   `env:"GRAPHITE_WRITE" envDefault:false`
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

// http status endpoint
func statusHandler(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(stats)
}

func main() {
	log.Info("Rabbit-carbon cassthrough starting")
	config := Config{}
	env.Parse(&config)
	if config.RabbitURI == "" {
		log.Errorf("No config found in ENVIRONMENT")
		os.Exit(1)
	}

	conn, err := amqp.Dial(config.RabbitURI)
	if err != nil {
		log.Criticalf("Could not connect to rabbit: %s", err)
	}

	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Criticalf("Could not open channel: %s\n", err)
	}

	g, err := graphite.NewGraphite(config.GraphiteHost, config.GraphitePort)
	if err != nil {
		log.Error("Could not connect to graphite, exiting")
		os.Exit(1)
	}

	// initiate stats values
	stats = make(map[string]int64)
	stats["ok"] = 0
	stats["errors"] = 0

	// start HTTP endpoint
	http.HandleFunc("/status", statusHandler)
	go func() {
		log.Infof("HTTP endpoint accepting requests on http://%s", config.StatusListen)
		if err := http.ListenAndServe(config.StatusListen, nil); err != nil {
			log.Warningf("Could not start HTTP endpoint (specified address: %s)", config.StatusListen)
		}
	}()

	// start a coroutine for each queue
	for _, queue := range config.RabbitQueues {
		msgs, err := ch.Consume(queue, config.RabbitTag, false, false, false, false, nil)
		if err != nil {
			log.Criticalf("Could not subscribe to queue: %s", err)
		}

		log.Infof("Bound to queue %s", queue)

		go func() {
			for m := range msgs {
				//log.Infof("Received message: %s", m.Body)
				// separate rabbit packet data
				s := string(m.Body[:])
				r := strings.Fields(s)

				// ugly conversion due to graphite wanting a struct with certain data
				data := r[0]
				value := r[1]

				timestamp, err := strconv.ParseInt(r[2], 10, 64)
				if err != nil {
					log.Warningf("Could not convert packet %s", err)
				}

				if len(r) > 3 {
					log.Infof("longer %s", len(r))
				}

				// make a struct for graphite sender
				message := graphite.Metric{data, value, timestamp}

				if err := g.SendMetric(message); err != nil {
					stats["errors"]++
				} else {
					if config.RabbitAck {
						// if RabbitAck is true,this removes the message from the queue
						m.Ack(false)
					}
					stats["ok"]++
				}

				// if there are more than configured numbers of errors, exit the process to let outside
				// environment handle restart/reconnects.
				if stats["errors"] > config.ExitErrors {
					os.Exit(1)
				}
			}
		}()
	}

	<-waiter
}
