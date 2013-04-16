#
#
# Note: to find xpath's (if this breaks), then use SelectorGadget.com (it will at least identify the class easily
#       You could just look at the html source also, unless it is complicated
#
#       If you use Firebug to get an XPath, you'll probably need to remove any "tbody" tags.  For some reason, it
#       adds them in places where Nokogiri doesn't expect to see them.
#
# If all goes well, you should end up with an HTML file with balance info in /tmp/output.html
#
# Ryan Chapman
#
# Thu Feb 2 04:11:07 MST 2011

require 'rubygems'
require 'logger'
require 'mechanize'
require 'pp'


# Be sure to fill in these values...

$SMTP_SERVER = 'localhost'

$USERID = "bill9123"

# Challenge questions are just regular expressions.  I prefer .*? non-greedy
$CHALLENGE_QUESTION1 = /high.*?school.*?graduated/
$CHALLENGE_ANSWER1 = 'valdosta'

$CHALLENGE_QUESTION2 = /maternal.*?grandfather.*?name/
$CHALLENGE_ANSWER2 = 'steve'

#$CHALLENGE_QUESTION3 = /year.*?graduate.*?college/
$CHALLENGE_QUESTION3 = /year.*?married/
$CHALLENGE_ANSWER3 = '1900'

$PASSWORD = 'MyPass123'

# download with 'curl http://curl.haxx.se/ca/cacert.pem > ~/bin/cacert.pem'
ca_file = File.expand_path "~/bin/cacert.pem"


# No need to modify anything below (hopefully)

$MACHINEATTR = 'colordepth=32|width=1266|height=768|availWidth=1366|availHeight=740|platform=Win32|javaEnabled=No|userAgent=Mozilla/4.0 (compatible; MSIE 7.0; Windows NT 5.1; Trident/4.0; .NET CLR 2.0.50727; .NET CLR 3.0.04506.648; .NET 3.5.21022)'


# I ended up sending mail from usbank_cron.sh script
def send_email(to, to_fullname, subject, message)
  from = "noreply@heatery.com"
  msg = <<vEOF
From: US Bank Daily Balance <#{from}>
To: ${to_fullname} <#{to}>
Subject: #{subject}

#{message}
vEOF
  Net::SMTP.start($SMTP_SERVER) do |smtp|
    smtp.send_message msg, from, to
  end
end



##########
## MAIN ##
##########

agent = Mechanize.new{|a| a.log = Logger.new(STDERR) }
agent.agent.http.ca_file = ca_path

# ENTRY PAGE
page = agent.get 'https://www4.usbank.com/internetBanking/RequestRouter?requestCmdId=DisplayLoginPage'

# debug -- print all forms
#pp page.forms

form = page.forms_with(:action => '/internetBanking/RequestRouter').first
form.field_with(:name => 'USERID').value = $USERID
form.field_with(:name => 'requestCmdId').value = 'VALIDATEID'
#form.field_with(:name => 'reqcrda').value = $USERID
#form.field_with(:name => 'reqcrdb').value = ''
#form.field_with(:name => 'NONCE').value = 'NoNonce'
#form.field_with(:name => 'MACHINEATTR').value = $MACHINEATTR
#form.field_with(:name => 'bankLogin').option_with(:value => 'internetBanking').select

page2 = agent.submit form


# CHALLENGE PAGE
challenge_question = page2.parser.xpath("//td[@class='f3']//text()").to_s
if (challenge_question =~ $CHALLENGE_QUESTION1)
  challenge_answer = $CHALLENGE_ANSWER1
elsif (challenge_question =~ $CHALLENGE_QUESTION2)
  challenge_answer = $CHALLENGE_ANSWER2
elsif (challenge_question =~ $CHALLENGE_QUESTION3)
  challenge_answer = $CHALLENGE_ANSWER3
elsif
  # ERROR: Unknown question
  # TODO: Send an email
  puts "ERROR: Got an unknown challenge question"
  puts "Challenge question was: '" + challenge_question + "'"
end
puts "Challenge question was: '" + challenge_question + "'"
puts "Challenge answer is...: '" + challenge_answer + "'"
form = page2.forms_with(:action => '/internetBanking/RequestRouter').first
form.field_with(:name => 'requestCmdId').value = 'VALIDATECHALLENGE'
form.field_with(:name => 'CHALLENGETYPE').value = 'QA'
form.field_with(:name => 'ANSWER').value = challenge_answer
form.field_with(:name => 'CHALLENGEANSWER').value = challenge_answer
form.add_field!('MACHINEATTR', $MACHINEATTR)
form.add_field!('doubleClick', '1')
form.add_field!('USEDSINGLEACCESSCODE', 'FALSE')
form.add_field!('EASTEPUPCHECKRESPONSESTEPUPREASON', 'ENROLLED')
# save the loginsessionid for password entry page (they don't include it in the form)
loginsessionid_field = form.field_with(:name => 'LOGINSESSIONID').value

page3 = agent.submit form


# PASSWORD ENTRY PAGE
#pp page3.body
form = page3.forms_with(:action => '/internetBanking/RequestRouter').first
form.field_with(:name => 'requestCmdId').value = 'Logon'
form.add_field!('PSWD', $PASSWORD)
form.add_field!('doubleClick', '2')
form.add_field!('USEDSINGLEACCESSCODE', 'FALSE')
form.add_field!('EASTEPUPCHECKRESPONSESTEPUPREASON', 'ENROLLED')
form.add_field!('LOGINSESSIONID', loginsessionid_field)

page4 = agent.submit form

# Page 4 could be an internet message.  View the message later.
form = page4.forms_with(:action => '/internetBanking/RequestRouter').first
if form.nil?
    balance_page = page4
else
    form.field_with(:name => 'requestCmdId').value = 'SubmitRIBNotification'
    form.field_with(:name => 'viewAgainLater').value = true
    page5 = agent.submit form
    balance_page = page5
end


# THE MEAT!  Balances, transactions, ...
f = File.open("/tmp/output.html", "w")
f.puts "<html><body>"
f.puts "<table>"

# table headers
#pp balance_page.body
balance_page.parser.xpath("/html/body/table[3]/tr/td[3]/table[2]").each do |line|
  f.puts line.to_s.gsub(/<\/?(img|a)[^>]*?>/i, "") 
end

# enumerate accounts and balances
#print "ENUM BALANCES:"
balance_page.parser.xpath("/html/body/table[3]/tr/td[3]/table[3]").each do |line|
  f.puts line.to_s.gsub(/<\/?(img|a)[^>]*?>/i, "") 
end

# done with balances
f.puts "</table>"


# print current pending transactions
#print "BALANCE PAGE:"
#pp balance_page
link = balance_page.parser.xpath("/html/body/table[3]/tr/td[3]/table[3]/tr[3]/td[3]/a").first
#print "LINK:"
#pp link
# mechanize doesn't appear to handle javascript: links, so manually handle for now
uri = ""
link.attributes.each do |k,v|
  if k == "onclick"
    uri = v.to_s.gsub(/^.*?\'|\'.*$/, "").gsub("requestCmdId=AccountDetails", "requestCmdId=DISPLAYAUTHORIZATIONS")
  end
end
pending_transactions_page = agent.get uri 

#print "\nPENDING TRANSACTIONS:\n"
#pp pending_transactions_page
pending_transactions_page.parser.xpath("/html/body/table[2]/tr/td[2]/table/tr[5]/td[2]/table").each do |line|
  f.puts line.to_s.gsub(/<\/?(img|a)[^>]*?>/i, "") 
end


# Done with output
f.puts "</html>"
f.close

# Email report
# TODO

