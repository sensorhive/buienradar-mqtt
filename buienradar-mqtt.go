/* A small Go program that will stuff data from the
 * [buienradar.nl](https://buienradar.nl)-API into your MQTT broker.
 *
 * This program will exit on any error, so be sure to run it in an init system
 * or other process manager.
 *
 * This program can also be ran through the use of containers, use either
 * `docker` or `podman`: `podman run -e MQTT_HOST="tcp://127.0.0.1:1883" quay.io/supakeen/mqtt-cron`
 *
 * Bug reports, feature requests can be filed at this projects homepage which
 * you can find at https://src.tty.cat/home.arpa/mqtt-cron
 *
 * This program was made by:
 * - Simon de Vlieger <cmdr@supakeen.com>
 *
 * This program is licensed under the MIT license:
 *
 * Copyright 2023 Simon de Vlieger
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy
 * of this software and associated documentation files (the "Software"), to
 * deal in the Software without restriction, including without limitation the
 * rights to use, copy, modify, merge, publish, distribute, sublicense,
 * and/or sell copies of the Software, and to permit persons to whom the
 * Software is furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in
 * all copies or substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
 * IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
 * FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
 * AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
 * LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING
 * FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER
 * DEALINGS IN THE SOFTWARE.
 */

package main

import (
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/eclipse/paho.mqtt.golang"
)

var logger = log.New(os.Stderr, "", log.LstdFlags)

/* Message passed along between *Loop and MessageLoop through a channel,
 * *Loop determines the data and where it goes. */
type MqttCronMessage struct {
	Topic   string
	Payload string
	Retain  bool
}

type BuienradarAPIStationData struct {
	Region string `xml:"regio,attr"`
	Name   string `xml:",chardata"`
}

type BuienradarAPIData struct {
	Code              string                   `xml:"stationcode"`
	Station           BuienradarAPIStationData `xml:"stationnaam"`
	Lat               string                   `xml:"lat"`
	Lon               string                   `xml:"lon"`
	Humidity          string                   `xml:"luchtvochtigheid"`
	TemperatureGround string                   `xml:"temperatuurGC"`
	Temperature10cm   string                   `xml:"temperatuur10cm"`
	WindSpeed         string                   `xml:"windsnelheidMS"`
	GustSpeed         string                   `xml:"windstotenMS"`
	AirPressure       string                   `xml:"luchtdruk"`
	SightRange        string                   `xml:"zichtmeters"`
	Rain              string                   `xml:"regenMMPU"`
}

type BuienradarAPIResult struct {
	XMLName  xml.Name            `xml:"buienradarnl"`
	Stations []BuienradarAPIData `xml:"weergegevens>actueel_weer>weerstations>weerstation"`
}

/* Result data from the `sunrise-sunset.org` API. */
type DayLightAPIData struct {
	Sunrise                   time.Time `json:"sunrise"`
	Sunset                    time.Time `json:"sunset"`
	SolarNoon                 time.Time `json:"solar_noon"`
	DayLength                 int       `json:"day_length"`
	CivilTwilightBegin        time.Time `json:"civil_twilight_begin"`
	CivilTwilightEnd          time.Time `json:"civil_twilight_end"`
	NauticalTwilightBegin     time.Time `json:"nautical_twilight_begin"`
	NauticalTwilightEnd       time.Time `json:"nautical_twilight_end"`
	AstronomicalTwilightBegin time.Time `json:"astronomical_twilight_begin"`
	AstronomicalTwilightEnd   time.Time `json:"astronomical_twilight_end"`
}

/* Result from the `sunrise-sunset.org` API. */
type DayLightAPIResult struct {
	Status  string          `json:"status"`
	Results DayLightAPIData `json:"results"`
}

/* Listens on a channel to submit messages to MQTT. */
func MessageLoop(c mqtt.Client, ch chan MqttCronMessage, prefix string) {
	for m := range ch {
		topic := fmt.Sprintf("%s/%s", prefix, m.Topic)

		if token := c.Publish(topic, 0, m.Retain, m.Payload); token.Wait() && token.Error() != nil {
			logger.Fatalln("MessageLoop could not publish message.")
		}

		logger.Printf("MessageLoop published topic='%s',payload='%s'\n", topic, m.Payload)
	}
}

/* The `buienradar.nl` API returns `-` when a value is not available, we convert
 * to empty string and check it later when queueing messages. */
func BuienradarAPINormalizeValue(value string) string {
	if value == "-" {
		return ""
	} else {
		return value
	}
}

/* Call the `buienradar.nl` API and return the array of station data. */
func BuienradarAPICall(apiUrl string) []BuienradarAPIData {
	var err error
	var res *http.Response

	if res, err = http.Get(apiUrl); err != nil {
		logger.Fatalln("BuienradarAPICall could not communicate with the `buienradar.nl` domain.")
	}

	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)

	if err != nil {
		logger.Fatalln("BuienradarAPICall could not read the response.")
	}

	var apiResult BuienradarAPIResult

	if err := xml.Unmarshal(body, &apiResult); err != nil {
		logger.Fatalln("BuienradarAPICall could not parse the response.")
	}

	return apiResult.Stations
}

func BuienradarLoop(ch chan MqttCronMessage) {
	topicFromEnv, topicExists := os.LookupEnv("MQTT_TOPIC")
	regionFromEnv, regionExists := os.LookupEnv("BUIENRADAR_REGION")

	if !topicExists {
		logger.Println("BuienradarLoop needs `MQTT_TOPIC` set in the environment, disabled.")
		return
	}

	if !regionExists {
		logger.Println("BuienradarLoop needs `BUIENRADAR_REGION` set in the environment, disabled.")
		return
	}

	for {
		for _, location := range BuienradarAPICall("https://data.buienradar.nl/1.0/feed/xml") {
			var msgs []string
			var tpcs []string

			regionName := strings.Replace(strings.ToLower(location.Station.Region), " ", "-", -1)

			if regionName != regionFromEnv {
				continue
			}

			if len(BuienradarAPINormalizeValue(location.Humidity)) > 0 {
				tpcs = append(tpcs, fmt.Sprintf("%s/%s", topicFromEnv, "humidity"))
				msgs = append(msgs, fmt.Sprintf("%s", location.Humidity))
			}

			if len(BuienradarAPINormalizeValue(location.TemperatureGround)) > 0 {
				tpcs = append(tpcs, fmt.Sprintf("%s/%s", topicFromEnv, "temperature.ground"))
				msgs = append(msgs, fmt.Sprintf("%s", location.TemperatureGround))
			}

			if len(BuienradarAPINormalizeValue(location.Temperature10cm)) > 0 {
				tpcs = append(tpcs, fmt.Sprintf("%s/%s", topicFromEnv, "temperature.10cm"))
				msgs = append(msgs, fmt.Sprintf("%s", location.Temperature10cm))
			}

			if len(BuienradarAPINormalizeValue(location.WindSpeed)) > 0 {
				tpcs = append(tpcs, fmt.Sprintf("%s/%s", topicFromEnv, "wind"))
				msgs = append(msgs, fmt.Sprintf("%s", location.WindSpeed))
			}

			if len(BuienradarAPINormalizeValue(location.GustSpeed)) > 0 {
				tpcs = append(tpcs, fmt.Sprintf("%s/%s", topicFromEnv, "gust"))
				msgs = append(msgs, fmt.Sprintf("%s", location.GustSpeed))
			}

			if len(BuienradarAPINormalizeValue(location.AirPressure)) > 0 {
				tpcs = append(tpcs, fmt.Sprintf("%s/%s", topicFromEnv, "pressure"))
				msgs = append(msgs, fmt.Sprintf("%s", location.AirPressure))
			}

			if len(BuienradarAPINormalizeValue(location.Rain)) > 0 {
				tpcs = append(tpcs, fmt.Sprintf("%s/%s", topicFromEnv, "rain"))
				msgs = append(msgs, fmt.Sprintf("%s", location.Rain))
			}

			if len(BuienradarAPINormalizeValue(location.SightRange)) > 0 {
				tpcs = append(tpcs, fmt.Sprintf("%s/%s", topicFromEnv, "sight"))
				msgs = append(msgs, fmt.Sprintf("%s", location.SightRange))
			}

			for idx, msg := range msgs {
				ch <- MqttCronMessage{Retain: false, Topic: tpcs[idx], Payload: msg}
			}
		}

		time.Sleep(5 * time.Minute)
	}
}

func main() {
	ch := make(chan MqttCronMessage)

	hostFromEnv, hostExists := os.LookupEnv("MQTT_HOST")

	if !hostExists {
		logger.Fatalln("mqtt-cron needs `MQTT_HOST` set in the environment to a value such as `tcp://127.0.0.1:1883`.")
	}

	prefixFromEnv, prefixExists := os.LookupEnv("MQTT_PREFIX")

	if !prefixExists {
		logger.Println("`MQTT_PREFIX` undefined using default `home.arpa`-prefix.")
		prefixFromEnv = "/home.arpa"
	} else {
		logger.Printf("`MQTT_PREFIX` set to `%s`.\n", prefixFromEnv)
	}

	opts := mqtt.NewClientOptions().AddBroker(hostFromEnv).SetClientID("mqtt-cron")
	opts.SetKeepAlive(2 * time.Second)
	opts.SetPingTimeout(1 * time.Second)

	c := mqtt.NewClient(opts)
	if token := c.Connect(); token.Wait() && token.Error() != nil {
		logger.Panic(token.Error())
	}

	go BuienradarLoop(ch)

	MessageLoop(c, ch, prefixFromEnv)

	c.Disconnect(250)

	time.Sleep(1 * time.Second)
}

// SPDX-License-Identifier: MIT
// vim: ts=4 sw=4 noet
