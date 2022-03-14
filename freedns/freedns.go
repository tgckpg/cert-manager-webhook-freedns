package freedns

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	logf "github.com/jetstack/cert-manager/pkg/logs"
	"golang.org/x/net/html"
)

type FreeDNSOperations interface {
	Login()
	SelectDomain()
	AddRecord()
	FindRecord()
	DeleteRecord()
}

type FreeDNS struct {
	AuthCookie *http.Cookie
	DomainId   string
}

const URI_LOGIN = "https://freedns.afraid.org/zc.php?step=2"
const URI_DOMAIN = "https://freedns.afraid.org/domain/"
const URI_ADD_RECORD = "https://freedns.afraid.org/subdomain/save.php?step=2"
const URI_SUBDOMAIN = "https://freedns.afraid.org/subdomain/?limit="
const URI_SUBDOMAIN_EDIT = "https://freedns.afraid.org/subdomain/edit.php?data_id="
const URI_LOGOUT = "https://freedns.afraid.org/logout/"
const URI_DELETE_RECORD = "https://freedns.afraid.org/subdomain/delete2.php?data_id[]=%s&submit=delete%%20selected"

func LogInfo(Mesg string) {
	// fmt.Println(Mesg)
	logf.V(logf.InfoLevel).Info(Mesg)
}

func LogDebug(Mesg string) {
	// fmt.Println(Mesg)
	logf.V(logf.DebugLevel).Info(Mesg)
}

func GetDomainFromZone(Zone string) string {
	_segs := strings.Split(strings.TrimSuffix(Zone, "."), ".")
	_segs = _segs[len(_segs)-2:]
	return strings.Join(_segs, ".")
}

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

func (dnsObj *FreeDNS) Logout() error {
	if dnsObj.AuthCookie == nil {
		return nil
	}

	_, _, err := _HttpRequest("GET", URI_LOGOUT, nil, dnsObj.AuthCookie)
	if err != nil {
		return err
	}

	dnsObj.AuthCookie = nil
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

	// Begin search for domain id
loop:
	for {
		tt := htmlTokens.Next()
		switch tt {
		case html.ErrorToken:
			break loop
		case html.TextToken:
			if inBold && strings.TrimSpace(htmlTokens.Token().Data) == DomainName {
				LogInfo("Found " + DomainName + ", looking for domain id")
				lookForA = true
			}
			// The [Manage] anchor is next to the bold tag
			//    <b>DOMAIN_NAME</b> <a href="">[Manage]</a>
		case html.StartTagToken:
			_t, hasAttr := htmlTokens.TagName()
			tagName := string(_t)
			inBold = tagName == "b"
			if lookForA && tagName == "a" && hasAttr {
				for {
					attrKey, attrValue, moreAttr := htmlTokens.TagAttr()
					_href := string(attrValue)
					if string(attrKey) == "href" && strings.HasPrefix(_href, "/subdomain/?limit=") {
						dnsObj.DomainId = strings.TrimPrefix(_href, "/subdomain/?limit=")
						LogDebug(fmt.Sprintf("Domain id for \"%s\" is %s\n", DomainName, dnsObj.DomainId))
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

		// Record already exists, treat this as success
		if strings.Contains(respStr, "already have another already existent") {
			LogInfo("Record already exists")
			return nil
		}

		// Try get a sense of the problem
		var errorMesgs []string
		lookForNextEl := 0
		lookForText := false
		_strBuffer := ""
		htmlTokens := html.NewTokenizer(strings.NewReader(respStr))
	loop:
		for {
			tt := htmlTokens.Next()
			switch tt {
			case html.ErrorToken:
				break loop
			case html.TextToken:
				_text := strings.TrimSpace(string(htmlTokens.Text()))
				// Search for the "1 error" / "N errors" message
				if strings.HasSuffix(_text, "error") || strings.HasSuffix(_text, "errors") {
					_text = strings.TrimSpace(strings.TrimSuffix(strings.TrimSuffix(_text, "s"), "error"))
					_n, _ := strconv.ParseInt(_text, 10, 8)
					// + 1 because we are already inside a font tag
					// The next closing </font> is ourself
					lookForNextEl = int(_n) + 1
				} else if lookForText {
					_strBuffer = _strBuffer + _text
				}

			case html.StartTagToken:
				_t, _ := htmlTokens.TagName()
				tagName := string(_t)
				if tagName == "font" && 0 < lookForNextEl {
					lookForText = true
					_strBuffer = ""
				}
			case html.EndTagToken:
				_t, _ := htmlTokens.TagName()
				tagName := string(_t)
				if tagName == "font" && 0 < lookForNextEl {
					lookForText = false
					errorMesgs = append(errorMesgs, strings.TrimSpace(_strBuffer))
					lookForNextEl--
				}
			}
		}

		if 0 < len(errorMesgs) {
			return errors.New(strings.Join(errorMesgs, ", "))
		}

		return errors.New("Unknown error while submitting record")
	}

	_Location, err := resp.Location()
	if err != nil {
		return err
	}

	if strings.HasPrefix(_Location.Path, "/zc.php") {
		LogDebug("Error on AddRecord: Cookie expired")
		return errors.New("dns_cookie maybe expired")
	}

	return nil
}

func (dnsObj *FreeDNS) DeleteRecord(RecordId string) error {
	resp, _, err := _HttpRequest("GET", fmt.Sprintf(URI_DELETE_RECORD, RecordId), nil, dnsObj.AuthCookie)
	if err != nil {
		return err
	}

	if resp.StatusCode != 302 {
		return errors.New("Unexpected " + fmt.Sprint(resp.StatusCode) + " from remote while deleting record")
	}

	_Location, err := resp.Location()
	if err != nil {
		return err
	}

	if strings.HasPrefix(_Location.Path, "/zc.php") {
		return errors.New("dns_cookie maybe expired")
	}

	return nil
}

func (dnsObj *FreeDNS) FindRecord(Subdomain string, RecordType string, Address string) (string, error) {
	if dnsObj.DomainId == "" {
		return "", errors.New("No domain selected")
	}

	resp, respStr, err := _HttpRequest("GET", URI_SUBDOMAIN+dnsObj.DomainId, nil, dnsObj.AuthCookie)
	if err != nil {
		return "", err
	}

	if resp.StatusCode == 302 {
		return "", errors.New("dns_cookie maybe expired")
	}

	var DeepSearchCandidates []string
	CurrRecordId := ""
	CurrRecordType := ""
	CurrRecordAddr := ""
	CurrTagName := ""
	lookForNextEl := 0

	htmlTokens := html.NewTokenizer(strings.NewReader(respStr))
loop:
	for {
		tt := htmlTokens.Next()
		switch tt {
		case html.ErrorToken:
			break loop
		case html.TextToken:
			if CurrTagName == "a" && lookForNextEl == 1 && CurrRecordAddr == "" {
				CurrRecordAddr = strings.TrimSpace(string(htmlTokens.Text()))
			} else if CurrTagName == "td" {
				if lookForNextEl == 1 {
					CurrRecordType = string(htmlTokens.Text())
					lookForNextEl = 2
				} else if lookForNextEl == 2 {
					_Addr := string(htmlTokens.Text())
					if CurrRecordType == RecordType && CurrRecordAddr == Subdomain {
						if _Addr == Address {
							return CurrRecordId, nil
						} else if strings.HasSuffix(_Addr, "...") && strings.HasPrefix(Address, strings.TrimSuffix(_Addr, "...")) {
							DeepSearchCandidates = append(DeepSearchCandidates, CurrRecordId)
						}
					}
					lookForNextEl = 0
				}
			}
		/** Each record is displayed with the following structure
		 *   <td bgcolor="#eeeeee">
		 *       <a href="edit.php?data_id=0000000">
		 *          [DOMAIN_NAME]
		 *       </a> (<b><font color="blue">G</font></b>)
		 *   </td>
		 *   <td bgcolor="#eeeeee">TXT</td>
		 *   <td bgcolor="#eeeeee">"google-site-verification=truncated_text...</td>
		 */
		case html.StartTagToken:
			_t, hasAttr := htmlTokens.TagName()
			CurrTagName = string(_t)
			if CurrTagName == "a" && hasAttr {
				for {
					attrKey, attrValue, moreAttr := htmlTokens.TagAttr()
					_href := string(attrValue)
					if string(attrKey) == "href" && strings.Contains(_href, "edit.php?data_id=") {
						lookForNextEl = 1
						CurrRecordAddr = ""
						CurrRecordId = strings.TrimPrefix(_href, "edit.php?data_id=")
						break
					}
					if !moreAttr {
						break
					}
				}
			}

		}
	}

	// Begin deep search for truncated records
	htmlAddr := strings.ReplaceAll(html.EscapeString(Address), "&#34;", "&quot;")
	for _, RecordId := range DeepSearchCandidates {
		LogDebug("Searching in " + RecordId)
		_, respStr, err := _HttpRequest("GET", URI_SUBDOMAIN_EDIT+RecordId, nil, dnsObj.AuthCookie)
		if err != nil {
			continue
		}

		if strings.Contains(respStr, htmlAddr) {
			return RecordId, nil
		}
	}

	return "", errors.New("No such record")
}
