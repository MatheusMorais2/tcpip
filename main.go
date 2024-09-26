package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type MacVendorResponseBody struct {
	StartHex  string `json:"startHex"`
	EndHex    string `json:"endHex"`
	StartDec  string `json:"startDec"`
	EndDec    string `json:"endDec"`
	Company   string `json:"company"`
	AddressL1 string `json:"addressL1"`
	AddressL2 string `json:"addressL2"`
	AddressL3 string `json:"addressL3"`
	Country   string `json:"country"`
	Type      string `json:"type"`
}

type PingData struct {
	IpAddress      string
	Url            string
	PacketLoss     float32
	MinLatency     time.Duration
	MaxLatency     time.Duration
	AvgLatency     time.Duration
	MeanDevLatency time.Duration
}

func main() {
	addresses, err := getAllMacAddresses()
	if err != nil {
		return
	}
	for i := 0; i < len(addresses); i++ {
		getMacAddressDetails(addresses[i])
	}
	pingData, err := PingUrl("google.com", 3)
	if err != nil {
		fmt.Println("error on ping: ", err)
		return
	}
	fmt.Printf("Ping data: %+v\n", pingData)
}

func getAllMacAddresses() ([]string, error) {
	cmd := exec.Command("arp", "-a")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("error executing arp: %+v", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("Error during command start: %+v", err)
	}
	var MacAddresses []string
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		result := scanner.Text()
		address, err := TrimUnecessaryTextFromArp(result)
		if err != nil {
			return nil, fmt.Errorf("Error formating address: %+v", err)
		}
		MacAddresses = append(MacAddresses, address)
	}

	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("error during waiting for the command: %+v", err)
	}
	return MacAddresses, nil
}

func TrimUnecessaryTextFromArp(result string) (string, error) {
	left, right, found := strings.Cut(result, ") at ")
	if !found {
		return "", fmt.Errorf("error getting arp list")
	}
	left, right, found = strings.Cut(right, " [")
	if !found {
		fmt.Println("error getting arp list")
		return "", fmt.Errorf("error getting arp list")
	}
	return left, nil
}

func getMacAddressDetails(mac string) {
	url := fmt.Sprintf("https://www.macvendorlookup.com/api/v2/%s", mac)
	request, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		fmt.Println("Error making request: ", err)
		return
	}

	client := &http.Client{}
	response, error := client.Do(request)
	if error != nil {
		fmt.Println("Error in the request: ", error)
		return
	}

	responseBody, error := io.ReadAll(response.Body)
	if error != nil {
		fmt.Println("Error reading response body: ", error)
	}

	var data []MacVendorResponseBody
	err = json.Unmarshal(responseBody, &data)
	if err != nil {
		fmt.Println("Error making request: ", err)
		return
	}
	fmt.Printf("response body: %+v\n", data[0])
}

func PingUrl(url string, numberOfPings int) (*PingData, error) {
	cmd := exec.Command("ping", "-c", strconv.Itoa(numberOfPings), url)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	scanner := bufio.NewScanner(stdout)
	i := 0
	ipData := PingData{}
	ipData.Url = url
	for scanner.Scan() {
		result := scanner.Text()
		fmt.Println("result: ", result)
		if i == 0 {
			ipData.IpAddress = getIpAddress(result)
			i++
		}

		if i == 3 {
			err := getDurations(result, &ipData)
			if err != nil {
				return nil, err
			}
		}

		if i == 2 {
			packetLoss, err := getPacketLossPercentage(result)
			if err != nil {
				return nil, err
			}
			ipData.PacketLoss = packetLoss
			i++
		}

		if strings.Contains(result, "---") {
			i++
		}
	}
	
	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("error during waiting for the command: %+v", err)
	}

	return &ipData, nil
}

func getIpAddress(result string) string {
	subStrings := strings.Split(result, " ")
	ipAddress := subStrings[2]
	ipAddress = strings.TrimLeft(ipAddress, "(")
	ipAddress = strings.TrimRight(ipAddress, ")")
	return ipAddress
}

func getPacketLossPercentage(result string) (float32, error) {
	substrings := strings.Split(result, ",")
	packetLoss := strings.Split(substrings[2], " ")
	numberStr := strings.TrimRight(packetLoss[1], "%")
	packetLossInt, err := strconv.Atoi(numberStr)
	if err != nil {
		return -1, err
	}
	return float32(packetLossInt) / 100, nil
}

func getDurations(result string, ipData *PingData) (err error) {
	substrings := strings.Split(result, " ")
	durations := strings.Split(substrings[3], "/")

	min, err := time.ParseDuration(durations[0] + "ms")
	if err != nil {
		return err
	}
	ipData.MinLatency = min

	avg, err := time.ParseDuration(durations[1] + "ms")
	if err != nil {
		return err
	}
	ipData.AvgLatency = avg
	
	max, err := time.ParseDuration(durations[2] + "ms")
	if err != nil {
		return err
	}
	ipData.MaxLatency = max

	mDev, err := time.ParseDuration(durations[3] + "ms")
	if err != nil {
		return err
	}
	ipData.MeanDevLatency = mDev

	return nil
}
