/*
 * Get current/available balance/pending transaction list from US Bank website
 *
 * To use, change the following constants below:
 *   USERNAME - your username for USBank.com online banking
 *   PASSWORD - online banking password
 *   CHALLENGE_QUESTION1 - Your ID Shield question 1
 *   CHALLENGE_ANSWER1   - Answer to ID Shield question 1
 *   CHALLENGE_QUESTION2 - Your ID Shield question 3
 *   CHALLENGE_ANSWER2   - Answer to ID Shield question 2
 *   CHALLENGE_QUESTION3 - Your ID Shield question 3
 *   CHALLENGE_ANSWER3   - Answer to ID Shield question 3
 *   CHALLENGE_QUESTION4 - Your ID Shield question 4
 *   CHALLENGE_ANSWER4   - Answer to ID Shield question 4
 *   CHALLENGE_QUESTION5 - Your ID Shield question 5
 *   CHALLENGE_ANSWER5   - Answer to ID Shield question 5
 *   and if you use stathat, then
 *   STATHAT_STATNAME    - passed to PostEZValue(statName, ..., ...)
 *   STATHAT_EZKEY       - passed to PostEZValue(..., ezkey, ...)
 *
 *  That's it!  
 *  Compile with "go build usbank.go"
 *  Execute this program with -outputFile FILE for where you want the HTML written
 *  You can then email FILE to yourself from a bash script. See usbank_cron2.sh
 *  for an example.
 *
 *
 * Note: golang1.1 is required (for cookie support)
 *
 * TODO: not sure if US Bank still shows a message page on login.  If so, that needs to be fixed in this program.
 *
 * Ryan A. Chapman, ryan@rchapman.org
 * Sat Apr 27 02:15:10 MDT 2013
 */

package main

import (
          "flag"
          "fmt"
          "github.com/moovweb/gokogiri"
    ghtml "github.com/moovweb/gokogiri/html"
    gxml  "github.com/moovweb/gokogiri/xml"
          "github.com/stathat/go"
          "html"
          "io"
          "io/ioutil"
          "net/http"
    cjar  "net/http/cookiejar"
          "net/url"
          "os"
          "regexp"
          "strconv"
          "strings"
)

const (
    USERNAME = "bill9123"
    PASSWORD = "MyPass123"

    // Leave alone if you don't use stathat
    STATHAT_STATNAME = ""
    STATHAT_EZKEY = ""

    // We support three to five possible challenge questions.  To find them, 
    //  sign into usbank.com, 
    //  click on "SECURITY CENTER", 
    //  click "View or Change Your Security Options", 
    //  answer a challenge question
    //  find your questions under "ID Shield Questions"
    // 
    // Leave challenge question/answers 4 and 5 empty is you only use 3 (the minimum)
    // Below, .*? is used where you find spaces in your questions
    CHALLENGE_QUESTION1 = `high.*?school.*?graduated`
    CHALLENGE_ANSWER1 = "valdosta"

    CHALLENGE_QUESTION2 = `maternal.*?grandfather.*?name`
    CHALLENGE_ANSWER2 = "steve"

    CHALLENGE_QUESTION3 = `year.*?graduate.*?college`
    CHALLENGE_ANSWER3 = "1900"

    CHALLENGE_QUESTION4 = ``
    CHALLENGE_ANSWER4 = ""

    CHALLENGE_QUESTION5 = ``
    CHALLENGE_ANSWER5 = ""

    // Shouldn't need to change anything below
    ROUTERURL   = "https://www4.usbank.com/internetBanking/RequestRouter"
    ENTRYPARAMS = "?requestCmdId=DisplayLoginPage"
    MACHINEATTR = "colordepth=32|width=1266|height=768|availWidth=1366|availHeight=740|platform=Win32|javaEnabled=No|userAgent=Mozilla/4.0 (compatible; MSIE 7.0; Windows NT 5.1; Trident/4.0; .NET CLR 2.0.50727; .NET CLR 3.0.04506.648; .NET 3.5.21022)"
    USERAGENT   = "Mozilla/4.0 (compatible; MSIE 7.0; Windows NT 5.1; Trident/4.0; .NET CLR 2.0.50727; .NET CLR 3.0.04506.648; .NET 3.5.21022)"
)

var client *http.Client

func httpReq(reqType string, url string, body io.Reader, pageName string) (*http.Response) {
    req, err := http.NewRequest(reqType, url, body)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error creating http request for %s page: %v\n", pageName, err)
        os.Exit(1)
    }
    req.Header.Set("User-Agent", USERAGENT)
    if reqType == "POST" {
        req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
    }
    if client == nil {
        jar, err := cjar.New(nil)
        if err != nil {
            fmt.Fprintf(os.Stderr, "Error creating cookie jar: %+v\n", err)
            os.Exit(1)
        }
        client = &http.Client{Jar: jar}
    }
    resp, err := client.Do(req)
    if err != nil || resp.StatusCode != 200 {
        fmt.Fprintf(os.Stderr, "Error submitting %s page:\n err=%+v\n resp=%+v)\n", pageName, err, resp)
        os.Exit(1)
    }
    return resp
}

func httpGet(url string, pageName string) (*http.Response) {
    resp := httpReq("GET", url, nil, pageName)
    return resp
}

func httpPost(url string, values url.Values, pageName string) (*http.Response) {
    body := strings.NewReader(values.Encode())
    resp := httpReq("POST", url, body, pageName)
    return resp
}

func parsePage(httpresp *http.Response, pageName string) (*ghtml.HtmlDocument) {
    page, err := ioutil.ReadAll(httpresp.Body)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error reading %s html body: %v\n", pageName, err)
        os.Exit(1)
    }
    httpresp.Body.Close()

    doc, err := gokogiri.ParseHtml(page)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error parsing %s html body: %v\n", pageName, err)
        os.Exit(1)
    }
    return doc
}

func docSearch(doc *ghtml.HtmlDocument, elementName string, pageName string, xpath string, mustFind bool) ([]gxml.Node) {
    elementArray, err := doc.Root().Search(xpath)
    if (err != nil || len(elementArray) == 0) && mustFind == false {
        return nil
    }
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error locating element \"%s\" in page %s (incorrect xpath?): %v\n", elementName, pageName, err)
        //fmt.Fprintf(os.Stderr, " doc=%+v\n", doc)
        os.Exit(1)
    }
    if len(elementArray) == 0 {
        fmt.Fprintf(os.Stderr, "Error locating element \"%s\" in page %s (incorrect xpath?): len() == 0\n", elementName, pageName)
        //fmt.Fprintf(os.Stderr, " doc=%+v\n", doc)
        os.Exit(1)
    }
    return elementArray
}

func getEntryPage() {
    // only reason to load entry page is to get the cookies in the cookie jar
    resp := httpGet(ROUTERURL + ENTRYPARAMS, "entry")
    resp.Body.Close()
}

func submitUsername() (*http.Response) {
    values := url.Values{"USERID":       {USERNAME}, 
                         "requestCmdId": {"VALIDATEID"},
                         "reqcrda":      {USERNAME},
                         "reqcrdb":      {""},
                         "NONCE":        {"NoNonce"},
                         "MACHINEATTR":  {MACHINEATTR},
                        }
    resp := httpPost(ROUTERURL, values, "submitUsername")
    return resp
}

func getChallengeAnswer(challengeQuestion string) (string) {
    re := regexp.MustCompile(CHALLENGE_QUESTION1)
    if re.MatchString(challengeQuestion) {
        return CHALLENGE_ANSWER1
    }
    re = regexp.MustCompile(CHALLENGE_QUESTION2)
    if re.MatchString(challengeQuestion) {
        return CHALLENGE_ANSWER2
    }
    re = regexp.MustCompile(CHALLENGE_QUESTION3)
    if re.MatchString(challengeQuestion) {
        return CHALLENGE_ANSWER3
    }
    re = regexp.MustCompile(CHALLENGE_QUESTION4)
    if re.MatchString(challengeQuestion) {
        return CHALLENGE_ANSWER4
    }
    re = regexp.MustCompile(CHALLENGE_QUESTION5)
    if re.MatchString(challengeQuestion) {
        return CHALLENGE_ANSWER5
    }
    fmt.Fprintf(os.Stderr, "Error determining challenge answer. Question asked by US Bank was \"%s\"\n", challengeQuestion)
    os.Exit(1)
    return ""  // never reached
}

func submitChallenge(usernameResp *http.Response) (*http.Response, string) {
    doc := parsePage(usernameResp, "challenge")

    xpath := `/html/body/table[3]/tr/td[3]/form/table[2]/tr/td/table/tr[2]/td[3]/table/tr[6]/td[2]/text()`
    mustFind := true
    challengeQuestion := fmt.Sprintf("%s", docSearch(doc, "challenge question", "challenge", xpath, mustFind)[0])

    xpath = `/html/body/table[3]/tr/td[3]/form/table[2]/tr/td/table/tr[2]/td[3]/table/tr[3]/td[3]/input/@value`
    loginSessionId := fmt.Sprintf("%s", docSearch(doc, "LOGINSESSIONID", "challenge", xpath, mustFind)[0])

    xpath = `//input[@type="hidden"][@name="BALDERDASH"]/@value`
    balderdash:= fmt.Sprintf("%s", docSearch(doc, "BALDERDASH", "challenge", xpath, mustFind)[0])

    doc.Free()

    challengeAnswer := getChallengeAnswer(challengeQuestion)

    values := url.Values{"requestCmdId":                      {"VALIDATECHALLENGE"},
                         "CHALLENGETYPE":                     {"QA"},
                         "ANSWER":                            {challengeAnswer},
                         "CHALLENGEANSWER":                   {challengeAnswer},
                         "MACHINEATTR":                       {MACHINEATTR},
                         "doubleClick":                       {"1"},
                         "USEDSINGLEACCESSCODE":              {"FALSE"},
                         "EASTEPUPCHECKRESPONSESTEPUPREASON": {"ENROLLED"},
                         "TYPE":                              {"ALPHANUM"},
                         "BALDERDASH":                        {balderdash},
                        }
   resp := httpPost(ROUTERURL, values, "challenge")
   return resp, loginSessionId
}

func submitPassword(challengeResp *http.Response, loginSessionId string) (*http.Response) {
    doc := parsePage(challengeResp, "password")

    xpath := `//input[@type="hidden"][@name="BALDERDASH"]/@value`
    mustFind := true
    balderdash:= fmt.Sprintf("%s", docSearch(doc, "BALDERDASH", "password", xpath, mustFind)[0])

    doc.Free()

    values := url.Values{"requestCmdId":                      {"Logon"},
                         "PSWD":                              {PASSWORD},
                         "LOGINSESSIONID":                    {loginSessionId},
                         "doubleClick":                       {"1"},
                         "USEDSINGLEACCESSCODE":              {"FALSE"},
                         "EASTEPUPCHECKRESPONSESTEPUPREASON": {"ENROLLED"},
                         "BALDERDASH":                        {balderdash},
                        }
    resp := httpPost(ROUTERURL, values, "password")
    return resp
}

func handleMessageToUser(passwordResp *http.Response) (*ghtml.HtmlDocument) {
    doc := parsePage(passwordResp, "passwordResponse")
    
    xpath := `//img[contains(@alt, 'View Again Later')]`
    mustFind := false
    found := docSearch(doc, "ViewAgainLater", "handleMessageToUser", xpath, mustFind)
    if found != nil {
        values := url.Values{"requestCmdId":                      {"SubmitRIBNotification"},
                             "NEWENTERPRISESESSION":              {"TRUE"},
                             "responseIdPostNotification":        {"DisplayAccountSummaryPage"},
                             "viewAgainLater":                    {"true"},
                            }
        resp := httpPost(ROUTERURL, values, "handleMessageToUser")

        doc = parsePage(resp, "handleMessageToUser_submitViewAgainLater")
    }

    return doc 
}

func printAccountsSummary(accountBalancesDoc *ghtml.HtmlDocument, outputFile *os.File) (*ghtml.HtmlDocument) {
    mustFind := true
    tableHeaders := fmt.Sprintf("%s", docSearch(accountBalancesDoc, "tableHeaders", "accountsSummary", `/html/body/table[3]/tr/td[3]/table[2]`, mustFind)[0])
    accountsAndBalances := fmt.Sprintf("%s", docSearch(accountBalancesDoc, "accountsAndBalances", "accountsSummary", `/html/body/table[3]/tr/td[3]/table[3]`, mustFind)[0])

    re := regexp.MustCompile(`</?(img|a)[^>]*?>`)
    tableHeaders = re.ReplaceAllLiteralString(tableHeaders, "")
    accountsAndBalances = re.ReplaceAllLiteralString(accountsAndBalances, "")
    fmt.Fprintf(outputFile, "<html>\n<body>\n%s\n%s\n\n", tableHeaders, accountsAndBalances)
    return accountBalancesDoc
}

func printPendingTransactions(doc *ghtml.HtmlDocument, outputFile *os.File) {
    xpath := `/html/body/table[3]/tr/td[3]/table[3]/tr[3]/td[3]/a`
    mustFind := true
    link := fmt.Sprintf("%s", docSearch(doc, "pendingTransactionsLink", "pendingTransactions", xpath, mustFind)[0])

    // pull the query string out of this: <a href="#" onclick="javascript:handlePageLink('/internetBanking/RequestRouter?requestCmdId=AccountDetails&amp;ACCOUNTLISTITEM=-2f504786%3A13e4de3653c%3A3401%7E117.20.52.58.221');return false;" name="accountInfo">Ryan Checking</a>
    re := regexp.MustCompile(`(^[^\?]*?)(\?[^\']*?)\'(.*?$)`)
    queryString := html.UnescapeString(re.ReplaceAllString(link, "$2"))

    // change requestCmdId to DISPLAYAUTHORIZATIONS to get to the pending transactions page
    re = regexp.MustCompile(`AccountDetails`)
    queryString = re.ReplaceAllLiteralString(queryString, "DISPLAYAUTHORIZATIONS")

    resp := httpGet(ROUTERURL + queryString, "pendingTransactions")
    doc = parsePage(resp, "pendingTransactionsTable")
    
    xpath = `/html/body/table[2]/tr/td[2]/table/tr[5]/td[2]/table`
    elementArray, _ := doc.Root().Search(xpath)	// make sure there are some pending transactions
    pendingTrxTable := ""
    if len(elementArray) != 0 {
        pendingTrxTable = fmt.Sprintf("%s", docSearch(doc, "pendingTransactionsTable", "pendingTransactions", xpath, mustFind)[0])
    }

    re = regexp.MustCompile(`</?(img|a)[^>]*?>`)
    pendingTrxTable = re.ReplaceAllLiteralString(pendingTrxTable, "")

    fmt.Fprintf(outputFile, "%s\n", pendingTrxTable)
}

var outputFile string
var help bool

func init() {
    flag.StringVar(&outputFile, "output", "<file>", "Output file for balance and pending transaction HTML")
    flag.BoolVar(&help, "help", false, "Show usage")
}

func usage() {
    fmt.Fprintf(os.Stderr, "usage: %s\n", os.Args[0])
    flag.PrintDefaults()
    os.Exit(1)
}

func postToStatHat(doc *ghtml.HtmlDocument) {
    xpath := `/html/body/table[3]/tr/td[3]/table[3]/tr[3]/td[13]/text()`
    mustFind := true
    checkingBalanceStr := fmt.Sprintf("%s", docSearch(doc, "checkingBalance", "postToStatHat", xpath, mustFind)[0])
    re := regexp.MustCompile(`([^0-9\.]*?)([0-9\.]+)`)
    checkingBalanceStr = re.ReplaceAllString(checkingBalanceStr, "$2")
    checkingBalance, err := strconv.ParseFloat(checkingBalanceStr, 64)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error converting checking balance into float: %v\n", err)
        os.Exit(1)
    }
    err = stathat.PostEZValue(STATHAT_STATNAME, STATHAT_EZKEY, checkingBalance)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error posting to stathat: %v\n", err)
    }
}


func main() {
    flag.Parse()
    if outputFile == "<file>" || help == true {
        usage()
    }
    getEntryPage()
    usernameResp := submitUsername()                                // returns challenge entry page
    challengeResp, loginSessionId := submitChallenge(usernameResp)  // returns password entry page
    passwordResp := submitPassword(challengeResp, loginSessionId)   // returns account balances page
    accountBalancesDoc := handleMessageToUser(passwordResp)         // returns account balances doc
    file, err := os.Create(outputFile)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error opening output file \"%s\" for writing: %v\n", outputFile, err)
        os.Exit(1)
    }
    doc := printAccountsSummary(accountBalancesDoc, file)
    printPendingTransactions(doc, file)
    if STATHAT_STATNAME != "" && STATHAT_EZKEY != "" {
        postToStatHat(doc)
    }
    doc.Free()
    fmt.Fprintf(file, "</html>\n")
    file.Close()
    fmt.Printf("Wrote account balances and pending transactions to %s\n", outputFile)
    os.Exit(0)
}

