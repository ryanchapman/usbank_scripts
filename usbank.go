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
    csv   "encoding/csv"
    json  "encoding/json"
          "flag"
          "fmt"
          "github.com/moovweb/gokogiri"
    ghtml "github.com/moovweb/gokogiri/html"
    gxml  "github.com/moovweb/gokogiri/xml"
          "io"
          "io/ioutil"
          "net/http"
    cjar  "net/http/cookiejar"
          "net/url"
          "os"
          "regexp"
          "strconv"
          "strings"
          "time"
)

const (
    USERNAME = "bill9123"
    PASSWORD = "MyPass123"

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
    CHALLENGE_ANSWER1 = "bozeman"

    CHALLENGE_QUESTION2 = `maternal.*?grandfather.*?name`
    CHALLENGE_ANSWER2 = "steve"

    CHALLENGE_QUESTION3 = `year.*?graduate.*?college`
    CHALLENGE_ANSWER3 = "1900"

    CHALLENGE_QUESTION4 = ``
    CHALLENGE_ANSWER4 = ""

    CHALLENGE_QUESTION5 = ``
    CHALLENGE_ANSWER5 = ""

    // Shouldn't need to change anything below
    ROUTERURL   = "https://onlinebanking.usbank.com"
    MACHINEATTR = "colorDepth=24|width=1440|height=900|availWidth=1387|availHeight=878|platform=MacIntel|javaEnabled=No|userAgent=Mozilla/5.0 (Macintosh; Intel Mac OS X 10.8; rv:30."
    USERAGENT   = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.8; rv:30."
)

type UserAndAccountsT struct {
    UserInfoResponse struct {
        FirstName string
        LastName string
        DateLastSignOn string
        DateLastSignOnString string
        Email string
    }
    AccountBalancesResponse []struct {
        Index float64
        DisplayName string
        AccountType string
        AccountNumber string
        CurrentBalanceSpecified bool
        CurrentBalance string
        CurrentBalanceString string
        AvailableBalanceSpecified bool
        AvailableBalance string
        AvailableBalanceString string
        AvailableBalanceValue float64
        RecentTrxDownloadStartDate string
        RecentTrxDownloadEndDate string
    }
}
var UserAndAccounts UserAndAccountsT 

type TransactionsT struct {
    Transactions []struct {
        Description string
        PostedAmount float64
        PostedAmountAsString string
        PostedDate string
        CardIdentifier string
    }
}
var PendingTransactions TransactionsT

var client *http.Client

func httpReq(reqType string, payloadType string, url string, referer string, body io.Reader, pageName string) (*http.Response) {
    req, err := http.NewRequest(reqType, url, body)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error creating http request for %s page: %v\n", pageName, err)
        os.Exit(1)
    }
    req.Header.Set("User-Agent", USERAGENT)
    if reqType == "POST" {
        if payloadType == "json" {
            req.Header.Set("Content-Type", "application/json")
        } else {
            req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
        }
    }
    if referer != "" {
        req.Header.Set("Referer", referer)
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
    resp := httpReq("GET", "", url, "", nil, pageName)
    return resp
}

func httpPost(url string, referer string, values url.Values, pageName string) (*http.Response) {
    body := strings.NewReader(values.Encode())
    resp := httpReq("POST", "", url, referer, body, pageName)
    return resp
}

func httpPostJson(url string, json string, pageName string) (*http.Response) {
    body := strings.NewReader(json)
    referer := ""
    resp := httpReq("POST", "json", url, referer, body, pageName)
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

func parseJson(httpresp *http.Response, pageName string) ([]byte) {
    page, err := ioutil.ReadAll(httpresp.Body)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error reading %s html body: %v\n", pageName, err)
        os.Exit(1)
    }
    httpresp.Body.Close()
    return page
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
    resp := httpGet(ROUTERURL, "entry")
    resp.Body.Close()
}

func submitUsername() (*http.Response) {
    values := url.Values{
                         "MachineAttribute":  {MACHINEATTR},
                         "PersonalId":        {USERNAME},
                        }
    resp := httpPost(ROUTERURL + "/Auth/Login/login", "", values, "submitUsername")
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

func submitChallenge(usernameResp *http.Response) (*http.Response) {
    doc := parsePage(usernameResp, "challenge")
    xpath := `//input[contains(@name, 'StepUpShieldQuestion.QuetionText')]/@value`
    mustFind := true
    challengeQuestion := fmt.Sprintf("%s", docSearch(doc, "challenge question", "challenge", xpath, mustFind)[0])

    doc.Free()

    challengeAnswer := getChallengeAnswer(challengeQuestion)

    values := url.Values{"MachineAttribute":                      {MACHINEATTR},
                         "StepUpShieldQuestion.Answer":           {challengeAnswer},
                         "StepUpShieldQuestion.AnswerFormat":     {"ALPHANUM"},
                         "StepUpShieldQuestion.AnswerMaxLength":  {"40"}, 
                         "StepUpShieldQuestion.QuestionText":     {challengeQuestion},
                         "StepUpShieldQuestion.RegisterComputer": {"false"},
                        }
   resp := httpPost(ROUTERURL + "/Auth/Login/StepUpCheck", "", values, "challenge")
   return resp
}

func submitPassword(challengeResp *http.Response) (*ghtml.HtmlDocument) {
    doc := parsePage(challengeResp, "password")
    doc.Free()

    values := url.Values{"userid":    {USERNAME},
                         "password":  {PASSWORD},
                        }
    resp := httpPost(ROUTERURL + "/access/oblix/apps/webgate/bin/webgate.dll?/Auth/Signon/Signon",
                     "https://onlinebanking.usbank.com/Auth/Login/Password", values, "password")
    accountBalancesDoc := parsePage(resp, "submitPassword")
    return accountBalancesDoc
}

func printAccountsSummary(accountBalancesDoc *ghtml.HtmlDocument, outputFile *os.File) (*ghtml.HtmlDocument) {
    mustFind := true
    pageJavascript := fmt.Sprintf("%s", docSearch(accountBalancesDoc, "CommonDataHelper.UserAndAccountsFromServer", "accountsSummary", `//script[contains(text(), 'CommonDataHelper.UserAndAccountsFromServer')]`, mustFind)[0])

    re := regexp.MustCompile(`CommonDataHelper\.UserAndAccountsFromServer.*`)
    userAndAccountsJsonArray := re.FindAllString(pageJavascript, 1)
    if userAndAccountsJsonArray == nil {
        fmt.Fprintf(os.Stderr, "Could not find CommonDataHelper.UserAndAccountsFromServer in any script tag within page")
        os.Exit(1)
    }
    userAndAccountsJson := userAndAccountsJsonArray[0]
   
    // Strip "CommonDataHelper.UserAndAccountsFromServer = " from beginning of json; also remove ending semicolon
    re = regexp.MustCompile(`CommonDataHelper\.UserAndAccountsFromServer\s*?=\s*?([^;]*?);`)
    userAndAccountsJson = re.ReplaceAllString(userAndAccountsJson, "$1")

    err := json.Unmarshal([]byte(userAndAccountsJson), &UserAndAccounts)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Could not deserialize UserAndAccounts JSON:\n err=%+v\n json=%+v\n)\n", err, userAndAccountsJson)
        os.Exit(1)
    }

    fmt.Fprintf(outputFile, "<html>\n<body>\n<h3>Deposit Accounts</h3>\n")
    fmt.Fprintf(outputFile, "<table width=586 border=0>\n<tr><td width=40%%>Account</td><td>Account Type</td>")
    fmt.Fprintf(outputFile, "<td align=right>Account<br>Balance</td><td align=right><b>Available<br>Balance</b></td></tr>\n")

    totalCurrentBalance, totalAvailableBalance := 0.00, 0.00
    for _, acct := range UserAndAccounts.AccountBalancesResponse {
        // skip check cards (PLAS) and loan accounts
        if acct.AccountType == "PLAS" || acct.AccountType == "INSL" {
            continue
        }
        availableBalance := acct.AvailableBalanceString
        // acct.AvailableBalanceValue is not provided by USBank, so we must calculate it with acct.AvailableBalanceString
        availableBalanceValue, err := strconv.ParseFloat(strings.Replace(strings.Replace(acct.AvailableBalanceString, "$", "", -1), ",", "", -1), 64)
        if err != nil {
            fmt.Fprintf(os.Stderr, "Could not convert AvailableBalanceString to an integer: err=%+v\n", err)
            os.Exit(1)
        }
        totalAvailableBalance += availableBalanceValue

        // acct.CurrentBalanceValue is not provided by USBank, so we must calculate it with acct.CurrentBalanceString
        currentBalanceValue, err := strconv.ParseFloat(strings.Replace(strings.Replace(acct.CurrentBalanceString, "$", "", -1), ",", "", -1), 64)
        if err != nil {
            fmt.Fprintf(os.Stderr, "Could not convert CurrentBalanceString to an integer: err=%+v\n", err)
            os.Exit(1)
        }
        totalCurrentBalance += currentBalanceValue
        fmt.Fprintf(outputFile, "<tr>\n")
        fmt.Fprintf(outputFile, "<td>%s</td>\n", acct.DisplayName)
        fmt.Fprintf(outputFile, "<td>%s</td>\n", acct.AccountType)
        fmt.Fprintf(outputFile, "<td align=right>%s</td>\n", acct.CurrentBalanceString)
        fmt.Fprintf(outputFile, "<td align=right>%s</td>\n", availableBalance)
        fmt.Fprintf(outputFile, "</tr>\n")
    }
    fmt.Fprintf(outputFile, "<tr>\n")
    fmt.Fprintf(outputFile, "<td colspan=2><b>TOTAL</b></td>\n")
    fmt.Fprintf(outputFile, "<td align=right><b>$%2.2f</b></td>\n", totalCurrentBalance)
    fmt.Fprintf(outputFile, "<td align=right><b>$%2.2f</b></td>", totalAvailableBalance)
    fmt.Fprintf(outputFile, "</tr>\n")
    fmt.Fprintf(outputFile, "</table>\n")
    return accountBalancesDoc
}

func printPendingTransactions(doc *ghtml.HtmlDocument, outputFile *os.File) (acctUrlKey string, acctIndexInt int) {
    mustFind := true
    pageJavascript := fmt.Sprintf("%s", docSearch(doc, "CDDashBoardHelper.urls.AccountDashboard", "pendingTransactions", `//script[contains(text(), 'CDDashBoardHelper.urls.AccountDashboard')]`, mustFind)[0])

    re := regexp.MustCompile(`CDDashBoardHelper\.urls\.AccountDashboard.*`)
    acctDashboardUrlArray := re.FindAllString(pageJavascript, 1)
    if acctDashboardUrlArray == nil {
        fmt.Fprintf(os.Stderr, "Could not find CDDashBoardHelper.urls.AccountDashboard in any script tag within page")
        os.Exit(1)
    }
    acctDashboardUrl := acctDashboardUrlArray[0]
   
    // We now have 'CDDashBoardHelper.urls.AccountDashboard = "/USB/af(51wu9DKg8Sf5bqSWTRi5)/AccountDashboard/Index";'
    // Strip all but "/USB/af(51wu9DKg8Sf5bqSWTRi5)/AccountDashboard/Index"
    //re = regexp.MustCompile(`CDDashBoardHelper\.urls\.AccountDashboard\s*?=\s*?([^;]*?);`)
    re = regexp.MustCompile(`[^/]*?/USB/([^/]*?)/.*?`)
    acctUrlKey = re.FindStringSubmatch(acctDashboardUrl)[1]
    if acctUrlKey == "" {
        fmt.Fprintf(os.Stderr, "Could not find key (usually looks like /USB/key/AccountDashboard/Index) in AccountDashboard URL \"%s\"\n", acctDashboardUrl)
        os.Exit(1)
    }

    var acctIndex float64 = -1 
    // find the index of first checking account
    for _, acct := range UserAndAccounts.AccountBalancesResponse {
        if acct.AccountType == "CHECKING" {
            //fmt.Printf("acctIndx=%f", acct.Index)
            acctIndex = acct.Index
        }
    } 
    // if we couldn't find a checking account, exit function
    if acctIndex == -1 {
        return
    }

    requestJson := fmt.Sprintf(`{"AccountIndex":%0.0f}`, acctIndex)
    url := fmt.Sprintf(`https://onlinebanking.usbank.com/USB/%s/AccountDashboard/GetCheckCardAuthorization`, acctUrlKey)
    resp := httpPostJson(url, requestJson, "pendingTransactions")
    pendingTransactionsJson := parseJson(resp, "pendingTransactions")

    err := json.Unmarshal(pendingTransactionsJson, &PendingTransactions)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Could not deserialize PendingTransactions JSON:\n err=%+v\n json=%+v\n)\n", err, pendingTransactionsJson)
        os.Exit(1)
    }

    fmt.Fprintf(outputFile, "<p><h3>Pending Transactions</h3></p>\n")
    fmt.Fprintf(outputFile, "<table width=586 border=0>\n")
    fmt.Fprintf(outputFile, "<tr><td>Date</td><td>Description</td><td>Card Ending In</td><td align=right>Amount Held</td></tr>\n")
    totalPending := 0.0
    for _, trx := range PendingTransactions.Transactions {
        fmt.Fprintf(outputFile, "<tr>\n")
        fmt.Fprintf(outputFile, "<td>%s</td>\n", trx.PostedDate)
        fmt.Fprintf(outputFile, "<td>%s</td>\n", trx.Description)
        fmt.Fprintf(outputFile, "<td align=center>%s</td>\n", trx.CardIdentifier)
        fmt.Fprintf(outputFile, "<td align=right>%s</td>\n", trx.PostedAmountAsString)
        fmt.Fprintf(outputFile, "</tr>\n")
      
        totalPending += trx.PostedAmount
    }
    fmt.Fprintf(outputFile, "<tr><td colspan=3 align=left><b>TOTAL</b></td><td align=right>$%2.2f</td></tr>\n", totalPending)
    fmt.Fprintf(outputFile, "</table>\n")
    acctIndexInt = int(acctIndex)
    return acctUrlKey, acctIndexInt
}

func printTransactionsFromLast3Days(acctUrlKey string, acctIndex int, outputFile *os.File) {
    now := time.Now()
    minus72h, err := time.ParseDuration("-72h")
    if err != nil {
        fmt.Fprintf(os.Stderr, "Could not figure out the date three days ago")
        os.Exit(1)
    }
    time3daysago := now.Add(minus72h)
    today := fmt.Sprintf("%02d/%02d/%04d", now.Month(), now.Day(), now.Year())
    threedaysago := fmt.Sprintf("%02d/%02d/%04d", time3daysago.Month(), time3daysago.Day(), time3daysago.Year())
    url := fmt.Sprintf(`https://onlinebanking.usbank.com/USB/%s/AccountDashboard/Download.aspx?index=%d&from=%s&to=%s&type=csv`,
                       acctUrlKey, acctIndex, threedaysago, today)
    resp := httpGet(url, "downloadTransactionsCSV")
    csvreader := csv.NewReader(resp.Body)
    transactions, err := csvreader.ReadAll()
    if err != nil {
        fmt.Fprintf(os.Stderr, "Could not download transactions as a CSV file. HTTP GET %s failed: %v\n", url, err)
        os.Exit(1)
    }
    resp.Body.Close()
    
    fmt.Fprintf(outputFile, "<p><h3>Transactions From Last Three Days</h3></p>\n")
    fmt.Fprintf(outputFile, "<table width=586 border=0>\n")
    //"Date","Transaction","Name","Memo","Amount"
    fmt.Fprintf(outputFile, "<tr><td>Date</td><td>Debit/Credit</td><td>Description</td><td align=right>Amount</td></tr>\n")
    for i, trx := range transactions {
        if i == 0 {
            continue
        }
        fmt.Fprintf(outputFile, "<tr>\n")
        fmt.Fprintf(outputFile, "<td>%s</td>\n", trx[0])
        fmt.Fprintf(outputFile, "<td>%s</td>\n", trx[1])
        fmt.Fprintf(outputFile, "<td>%s</td>\n", trx[2])
        fmt.Fprintf(outputFile, "<td align=right>%s</td>\n", trx[4])
        fmt.Fprintf(outputFile, "</tr>\n")
    }
    fmt.Fprintf(outputFile, "</table>\n")
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

func main() {
    flag.Parse()
    if outputFile == "<file>" || help == true {
        usage()
    }
    getEntryPage()
    usernameResp := submitUsername()                    // returns challenge entry page
    challengeResp := submitChallenge(usernameResp)      // returns password entry page
    accountBalancesDoc := submitPassword(challengeResp) // returns account balances page
    file, err := os.Create(outputFile)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Error opening output file \"%s\" for writing: %v\n", outputFile, err)
        os.Exit(1)
    }
    doc := printAccountsSummary(accountBalancesDoc, file)
    acctUrlKey, acctIndex := printPendingTransactions(doc, file)
    printTransactionsFromLast3Days(acctUrlKey, acctIndex, file)
    doc.Free()
    fmt.Fprintf(file, "</body></html>\n")
    file.Close()
    fmt.Printf("Wrote account balances and pending transactions to %s\n", outputFile)
    os.Exit(0)
}

