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
	"os"
	"strings"

	"github.com/sendgrid/rest"
)

type configuration struct {
	Endpoint string `json:"endpoint"`
	Port     string `json:"port"`
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
	return conf
}

func indexHandler(response http.ResponseWriter, request *http.Request) {
	_, err := fmt.Fprintf(response, "%s", "Hello World")
	if err != nil {
		log.Fatal(err)
	}
}

func inboundHandler(response http.ResponseWriter, request *http.Request) {
	log.Println(" ################## inboundHandler ##################")
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

				printMap(parsedEmail, "")

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

func printMap(inputMap map[string]string, prefix string) {
	for key, value := range inputMap {
		if key == "from" || key == "html" {
			fmt.Println(prefix, "Key:", key, " === ", prefix, "Value:", value)
		}
		if key == "from" {
			p1 := strings.Index(value, "<")
			log.Println("TRIM STRING ", value[p1:])
			//from email processing
		}
		if key == "html" {
			htmlString := value
			fmt.Println("htmlString ", htmlString)
			//html processing
		}
	}
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
