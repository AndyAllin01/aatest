//test captain API
/*
func main() {
	//CAPTAIN TEST:
	/*
		fmt.Println("CAPTAIN TEST")
		client := captain.NewClient()

		fmt.Printf("client %+v\n", client)

			instrs := "TEST SPECIAL INSTRS"
			order := &captain.Order{
				SpecialInstructions: &instrs,
			}
			fmt.Printf("order %+v\n", order)

			resp, err:=client.CreateOrder()



*/
//STANDARD SENDGRID PARSER
package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/buger/jsonparser"
	"github.com/sendgrid/rest"
)

const (
	//ReqStr is base URL used to build up the LocationIQ search request string
	ReqStr = "https://us1.locationiq.com/v1/search.php?"
)

var LocationIQKey string

type configuration struct {
	Endpoint      string `json:"endpoint"`
	Port          string `json:"port"`
	LocationIQKey string `json:"locationiqkey"`
}

func loadConfig(path string) configuration {
	file, err := ioutil.ReadFile(path)
	if err != nil {
		log.Fatal("Config File Missing. ", err)
	}
	var conf configuration
	err = json.Unmarshal(file, &conf)
	if err != nil {
		log.Fatal("Config Parse Error: ", err)
	}
	log.Println("########### CONFIG ##############", conf)
	return conf
}

func indexHandler(response http.ResponseWriter, request *http.Request) {
	_, err := fmt.Fprintf(response, "%s", "Hello World")
	if err != nil {
		log.Fatal(err)
	}
}

func inboundHandler(response http.ResponseWriter, request *http.Request) {
	mediaType, params, err := mime.ParseMediaType(request.Header.Get("Content-Type"))
	if err != nil {
		log.Fatal(err)
	}
	if strings.HasPrefix(mediaType, "multipart/") {
		mr := multipart.NewReader(request.Body, params["boundary"])
		parsedEmail := make(map[string]string)
		emailHeader := make(map[string]string)
		binaryFiles := make(map[string][]byte)

		for {
			p, err := mr.NextPart()
			// We have found an attachment with binary data
			if err == nil && p.FileName() != "" {
				contents, ioerr := ioutil.ReadAll(p)
				if ioerr != nil {
					log.Fatal(err)
				}
				binaryFiles[p.FileName()] = contents
			}
			if err == io.EOF {
				// We have finished parsing, do something with the parsed data
				fmt.Printf("PARSED EMAIL %+v\n", parsedEmail)

				oStruct := printMap(parsedEmail, "")

				log.Println("OUTPUT STRUCT ", oStruct)
				//populated basic struct with order details, now get coords
				LIQString := formatReqString(oStruct)
				log.Println("################### LIQSTRING ", LIQString)
				//get coords of delivery address from LocationIQ API call
				dLat, dLng, err := MakeRequest(LIQString)
				if err != nil {
					log.Println("LocationIQ error getting delivery coords")
				}
				log.Println("DELIVERY COORDS ", dLat, dLng)
				//all available fields from the email are now parsed into the OrderInfo struct
				//now need to convert into Captain Request struct

				//Write details to postgresql:
				// inbound html, json captain request, response, orderid, (time, customer?)

				// Twilio SendGrid needs a 200 OK response to stop POSTing
				response.WriteHeader(http.StatusOK)
				return
			}
			if err != nil {
				log.Fatal(err)
			}
			value, err := ioutil.ReadAll(p)
			if err != nil {
				log.Fatal(err)
			}
			header := p.Header.Get("Content-Disposition")
			if !strings.Contains(header, "filename") {
				header = header[17 : len(header)-1]
				parsedEmail[header] = string(value)
			} else {
				header = header[11:]
				f := strings.Split(header, "=")
				parsedEmail[f[1][1:len(f[1])-11]] = f[2][1 : len(f[2])-1]
			}

			if header == "headers" {
				handleHeaders(value, emailHeader)
			}
			// Since we have parsed the headers, we can delete the original
			delete(parsedEmail, "headers")

			// Since we've parsed this header, we can delete the original
			delete(parsedEmail, "email")
		}
	}
}

//formatReqString formats and returns a LocationIQ request URL based on the address in the
//parsed email
func formatReqString(o OrderInfo) string {
	baseURL, err := url.Parse(ReqStr)
	if err != nil {
		fmt.Println("Malformed URL: ", err.Error())
		return ""
	}
	params := url.Values{}
	params.Add("format", "json")
	log.Println("=================================", LocationIQKey)
	params.Add("key", LocationIQKey)
	log.Println("=================================", o.FullAddr)
	loc := o.FullAddr
	log.Println("=================================", loc)
	params.Add("q", loc)
	baseURL.RawQuery = params.Encode()
	return baseURL.String()
}

//MakeRequest sends request to LocationIQ service and returns the lat/lng coords for the address passed in
func MakeRequest(target string) (float64, float64, error) {
	resp, err := http.Get(target)
	//	resp, err := http.Get("https://us1.locationiq.com/v1/search.php?key=e397dae178546d&q=35%20Quarry%20Street,Geraldton,Australia&format=json")
	if err != nil {
		log.Printf("HTTP request to LocationIQ failed with %s\n", err)
		return 0, 0, err
	}
	respBody, _ := ioutil.ReadAll(resp.Body)

	//Might full address be useful?
	/*		addr, _, _, err := jsonparser.Get(respBody, "[0]", "display_name")
			if err != nil {
				fmt.Println("jsonparser error address", err)
			}*/
	lat, _, _, err := jsonparser.Get(respBody, "[0]", "lat")
	if err != nil {
		fmt.Println("jsonparser error lat ", err)
	}
	lng, _, _, err := jsonparser.Get(respBody, "[0]", "lon")
	if err != nil {
		fmt.Println("jsonparser error lng ", err)
	}

	delivLat, _ := strconv.ParseFloat(string(lat), 64)
	delivLng, _ := strconv.ParseFloat(string(lng), 64)

	fmt.Println("DELIV COORDS ", delivLat, delivLng)
	//	log.Println(string(respBody))
	return delivLat, delivLng, nil
}

func handleHeaders(value []byte, emailHeader map[string]string) {
	s := strings.Split(string(value), "\n")
	var a []string
	for _, v := range s {
		t := strings.Split(string(v), ": ")
		a = append(a, t...)
	}
	for i := 0; i < len(a)-1; i += 2 {
		emailHeader[a[i]] = a[i+1]
	}
}

func printMap(inputMap map[string]string, prefix string) OrderInfo {
	var str, htmlString string
	for key, value := range inputMap {
		if key == "from" || key == "html" {
			fmt.Println(prefix, "Key:", key, " === ", prefix, "Value:", value)
		}
		if key == "from" {
			p1 := strings.Index(value, "<")
			str = value[p1+1:]
			str = str[:len(str)-1]
			log.Println("TRIMMED STRING ", str)
			//from email processing
		}
		if key == "html" {
			htmlString = value
			fmt.Println("htmlString ", htmlString)
			//html processing
		}
	}
	filledStruct := composeStruct(htmlString, str)
	return filledStruct
}

//composeStruct receives a string of html and parses this into readable
//key/value map which is then passed to fillStruct function
func composeStruct(str, oEmail string) OrderInfo {
	htmlMap := make(map[string]string)
	//chop up HTML by <br>
	stringSlice := strings.Split(str, "<br>")
	//for each "line", see if it contains a key/value pair
	for i := 0; i < len(stringSlice); i++ {
		if strings.Contains(stringSlice[i], ": <") {
			//don't lose zip code by splitting on ":" so replace with space
			stringSlice[i] = strings.Replace(stringSlice[i], "zip:", "zip ", -1)
			keyVal := strings.Split(stringSlice[i], ": ")
			//strip out <b>
			key := strings.Replace(keyVal[0], "<b>", "", -1)
			val := strings.Replace(keyVal[1], "<b>", "", -1)
			//remove anything after and incl </b>
			valx := strings.Split(val, "</b>")
			val = valx[0]
			//remove and trim <span> formatting
			if strings.Contains(val, "span") {
				//remove up to 1st > and after 2nd <
				i := strings.Index(val, ">")
				val = val[i+1:]
				i = strings.Index(val, "<")
				val = val[:i]
			}
			if key == "Order by" {
				//split order by field into name and number
				oName := strings.Split(val, " - ")
				htmlMap["OrderName"] = oName[0]
				htmlMap["OrderNumber"] = oName[1]
			} else if key == "Delivery address" {
				//split order by field into address components
				htmlMap["FullAddr"] = val
				val += ",,,,,,,,"
				oAddr := strings.Split(val, ",")
				htmlMap["Addr1"] = oAddr[0]
				htmlMap["Addr2"] = oAddr[1]
				htmlMap["Addr3"] = oAddr[2]
				htmlMap["Addr4"] = oAddr[3]
				htmlMap["Addr5"] = oAddr[4]
				htmlMap["Addr6"] = oAddr[5]
				htmlMap["Addr7"] = oAddr[6]
				htmlMap["Addr8"] = oAddr[7]
			} else {
				htmlMap[key] = val
			}
		}
	}
	order := fillStruct(htmlMap, oEmail)
	//	fmt.Println(order)
	return order
}

//fillStruct populates the orderInfo struct with elements parsed from the received HTML
func fillStruct(m map[string]string, oEmail string) OrderInfo {
	//convert string fields from the html into float64
	fDeliveryPrice, _ := strconv.ParseFloat(m["Delivery price"], 64)
	fDeliveryTip, _ := strconv.ParseFloat(m["Delivery tip"], 64)
	fCash, _ := strconv.ParseFloat(m["Cash"], 64)
	fSubTotal, _ := strconv.ParseFloat(m["Sub Total"], 64)
	fTax, _ := strconv.ParseFloat(m["Tax"], 64)
	fTotal, _ := strconv.ParseFloat(m["Total"], 64)

	d := DelivAddr{
		Addr1: m["Addr1"],
		Addr2: m["Addr2"],
		Addr3: m["Addr3"],
		Addr4: m["Addr4"],
		Addr5: m["Addr5"],
		Addr6: m["Addr6"],
		Addr7: m["Addr7"],
		Addr8: m["Addr8"],
	}

	//format OrderInfo struct
	o := OrderInfo{
		TransactionID:           m["Transaction Id"],
		AccountIdentifier:       oEmail,
		DeliveryTime:            m["Delivery time"],
		FullAddr:                m["FullAddr"],
		DeliveryAddress:         d,
		DeliveryAddressComments: m["Delivery address comments"],
		Size:                    m["Size"],
		DeliveryPrice:           fDeliveryPrice,
		DeliveryTip:             fDeliveryTip,
		Cash:                    fCash,
		SubTotal:                fSubTotal,
		Tax:                     fTax,
		Total:                   fTotal,
		OrderName:               m["OrderName"],
		OrderNumber:             m["OrderNumber"],
		OrderTimeOfCust:         m["Order time of customer"],
	}
	return o
}

//DelivAddr contains up to 8 lines of address
type DelivAddr struct {
	Addr1 string
	Addr2 string
	Addr3 string
	Addr4 string
	Addr5 string
	Addr6 string
	Addr7 string
	Addr8 string
}

//OrderInfo contains details scraped from the email (parsed by sendgrid)
type OrderInfo struct {
	TransactionID           string
	AccountIdentifier       string
	DeliveryTime            string
	FullAddr                string
	DeliveryAddress         DelivAddr
	DeliveryAddressComments string
	Size                    string
	DeliveryPrice           float64
	DeliveryTip             float64
	Cash                    float64
	SubTotal                float64
	Tax                     float64
	Total                   float64
	OrderName               string
	OrderNumber             string
	OrderTimeOfCust         string
}

func main() {
	if len(os.Args) > 1 {
		// Test Sender
		path := os.Args[1]
		host := os.Args[2]
		file, err := ioutil.ReadFile(path)
		if err != nil {
			log.Fatal("Check your Filepath. ", err)
		}
		Headers := map[string]string{
			"User-Agent":   "Twilio-SendGrid-Test",
			"Content-Type": "multipart/form-data; boundary=xYzZY",
		}
		method := rest.Post
		request := rest.Request{
			Method:  method,
			BaseURL: host,
			Headers: Headers,
			Body:    file,
		}
		_, err = rest.Send(request)
		if err != nil {
			log.Fatal("Check your Filepath. ", err)
		}
	} else {
		conf := loadConfig("./conf.json")
		LocationIQKey = conf.LocationIQKey
		http.HandleFunc("/", indexHandler)
		http.HandleFunc(conf.Endpoint, inboundHandler)
		port, err := determineListenAddress()
		if err != nil {
			log.Println("error detecting port")
		}
		//port := os.Getenv("PORT")
		fmt.Println("ENVIRNMENT PORT :", port)
		if port == "" {
			port = conf.Port
		}
		if err := http.ListenAndServe(port, nil); err != nil {
			log.Fatalln("ListenAndServe Error", err)
		}
	}
}

func determineListenAddress() (string, error) {
	port := os.Getenv("PORT")
	if port == "" {
		return "", errors.New("unable to get port information")
	}
	return ":" + port, nil
}
