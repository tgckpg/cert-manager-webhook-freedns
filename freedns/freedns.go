package freedns

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"golang.org/x/net/html"
)

type FreeDNSOperations interface {
	Login()
	SelectDomain()
	AddRecord()
}

type FreeDNS struct {
	AuthCookie *http.Cookie
	DomainId   string
}

const URI_LOGIN string = "https://freedns.afraid.org/zc.php?step=2"
const URI_DOMAIN string = "https://freedns.afraid.org/domain/"
const URI_ADD_RECORD string = "https://freedns.afraid.org/subdomain/save.php?step=2"

// const URI_LOGIN string = "http://127.0.0.1:1234/"

func _HttpRequest(method string, url string, PostData url.Values, ExCookie *http.Cookie) (*http.Response, string, error) {
	client := http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	var req *http.Request
	var err error

	if method == "GET" {
		req, err = http.NewRequest(method, url, nil)
	} else if method == "POST" {
		req, err = http.NewRequest(method, url, strings.NewReader(PostData.Encode()))
		req.Header.Add("Content-Type", "application/x-www-form-urlencoded")
	} else {
		return nil, "", errors.New("Method + \"" + method + "\" is not supported")
	}

	if err != nil {
		return nil, "", err
	}

	if ExCookie != nil {
		req.AddCookie(ExCookie)
	}

	resp, err := client.Do(req)
	if err != nil {
		return resp, "", err
	}

	respData, err := ioutil.ReadAll(resp.Body)
	resp.Body.Close()

	if err != nil {
		return resp, "", err
	}

	return resp, string(respData), nil
}

func (dnsObj *FreeDNS) Login(Username string, Password string) error {

	authData := url.Values{}
	authData.Set("username", Username)
	authData.Set("password", Password)
	authData.Set("submit", "Login")
	authData.Set("action", "auth")

	resp, respString, err := _HttpRequest("POST", URI_LOGIN, authData, nil)
	if err != nil {
		return err
	}

	if strings.Contains(respString, "Invalid UserID/Pass") {
		return errors.New("Invalid UserID/Pass")
	}

	for _, cookie := range resp.Cookies() {
		if cookie.Name == "dns_cookie" {
			dnsObj.AuthCookie = cookie
		}
	}

	return nil
}

func (dnsObj *FreeDNS) SelectDomain(DomainName string) error {
	if dnsObj.AuthCookie == nil {
		return errors.New("Not logged in")
	}

	resp, respStr, err := _HttpRequest("GET", URI_DOMAIN, nil, dnsObj.AuthCookie)
	if err != nil {
		return err
	}

	if resp.StatusCode == 302 {
		return errors.New("dns_cookie maybe expired")
	}

	htmlTokens := html.NewTokenizer(strings.NewReader(respStr))

	inBold := false
	lookForA := false
	dnsObj.DomainId = ""
loop:
	for {
		tt := htmlTokens.Next()
		switch tt {
		case html.ErrorToken:
			break loop
		case html.TextToken:
			if inBold && strings.TrimSpace(htmlTokens.Token().Data) == DomainName {
				fmt.Println("Found " + DomainName + ", looking for domain id")
				lookForA = true
			}
		case html.StartTagToken:
			_t, hasAttr := htmlTokens.TagName()
			tagName := string(_t)
			inBold = tagName == "b"
			if lookForA && tagName == "a" && hasAttr {
				for {
					attrKey, attrValue, moreAttr := htmlTokens.TagAttr()
					_href := string(attrValue)
					if string(attrKey) == "href" && strings.Contains(_href, "/subdomain/?limit=") {
						dnsObj.DomainId = strings.TrimLeft(_href, "/subdomain/?limit=")
						fmt.Printf("Domain id for \"%s\" is %s\n", DomainName, dnsObj.DomainId)
						break loop
					}
					if !moreAttr {
						break
					}
				}
			}
		}
	}

	if dnsObj.DomainId == "" {
		return errors.New(fmt.Sprintf("Unable to locate domain id for \"%s\" under /domain/ page", DomainName))
	}

	return nil
}

func (dnsObj *FreeDNS) AddRecord(RecordType string, Subdomain string, Address string, Wildcard bool, ttl string) error {
	if dnsObj.DomainId == "" {
		return errors.New("No domain selected")
	}
	recordData := url.Values{}
	recordData.Set("type", RecordType)
	recordData.Set("domain_id", dnsObj.DomainId)
	recordData.Set("subdomain", Subdomain)
	recordData.Set("address", Address)
	recordData.Set("send", "Save!")
	if Wildcard {
		recordData.Set("wildcard", "1")
	}

	resp, respStr, err := _HttpRequest("POST", URI_ADD_RECORD, recordData, dnsObj.AuthCookie)
	if err != nil {
		return err
	}

	if resp.StatusCode != 302 {

		// Record already exists, treated as success
		if strings.Contains(respStr, "already have another already existent") {
			fmt.Println("Record already exists")
			return nil
		}

		htmlTokens := html.NewTokenizer(strings.NewReader(respStr))
	loop:
		for {
			tt := htmlTokens.Next()
			switch tt {
			case html.ErrorToken:
				break loop
			case html.TextToken:
			case html.StartTagToken:
			}
		}

		return errors.New("Unknown error while submitting record")
	}

	_Location, err := resp.Location()
	if err != nil {
		return err
	}

	if strings.Contains(_Location.Path, "/zc.php") {
		fmt.Println("Error on AddRecord: Cookie expired")
		return errors.New("Cookie exipired")
	}

	return nil
}
