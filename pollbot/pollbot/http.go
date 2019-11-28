package pollbot

import (
	"fmt"
	"net/http"

	"github.com/keybase/go-keybase-chat-bot/kbchat"
	"github.com/keybase/go-keybase-chat-bot/kbchat/types/chat1"
)

type HTTPSrv struct {
	kbc *kbchat.API
	db  *DB
}

func NewHTTPSrv(kbc *kbchat.API, db *DB) *HTTPSrv {
	return &HTTPSrv{
		kbc: kbc,
		db:  db,
	}
}

func (h *HTTPSrv) debug(msg string, args ...interface{}) {
	fmt.Printf("HTTPSrv: "+msg+"\n", args...)
}

func (h *HTTPSrv) getConvName(conv chat1.ConvSummary) string {
	name := conv.Channel.Name
	if conv.Channel.MembersType == "team" {
		name += "#" + conv.Channel.TopicName
	}
	return name
}

func (h *HTTPSrv) handleConfirm(w http.ResponseWriter, r *http.Request) {
	vstr := r.URL.Query().Get("vote")
	username := r.URL.Query().Get("username")
	vote := NewVoteFromEncoded(vstr)
	conv, err := h.kbc.GetConversation(vote.ConvID)
	if err != nil {
		h.debug("failed to get conv: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	body := fmt.Sprintf(`Thank for you for voting! This message corresponds to your pick of option (%d) in a recent anonymous poll in %s. If you did not vote in this poll please ignore me, someone probably just made a mistake.

*In order to authenticate your vote, please hit the green and white checkmark below to add a reaction.*`, vote.Choice, h.getConvName(conv))

	sendRes, err := h.kbc.SendMessageByTlfName(username, body)
	if err != nil {
		h.debug("failed to send msg: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if sendRes.Result.MessageID == nil {
		h.debug("no msgid returned")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if err := h.db.StageVote(username, *sendRes.Result.MessageID, vote); err != nil {
		h.debug("failed to stage vote: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if _, err := h.kbc.ReactByChannel(chat1.ChatChannel{
		Name: username,
	}, *sendRes.Result.MessageID, ":white_check_mark:"); err != nil {
		h.debug("failed to set reaction: %s", err)
	}
}

func (h *HTTPSrv) handleVote(w http.ResponseWriter, r *http.Request) {
	vstr := r.URL.Query().Get("")
	vote := NewVoteFromEncoded(vstr)
	conv, err := h.kbc.GetConversation(vote.ConvID)
	if err != nil {
		h.debug("failed to get conv: %s", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	body := fmt.Sprintf(`
	<html>
		<head>
			<title>Polling Service Confirmation</title>
			<script>
				window.disableSubmit = function() {
					var btn = document.getElementById("btnSubmit");
					btn.onclick = undefined;
					btn.style.cursor = 'text';
					btn.style.backgroundColor = 'gray';
				}
				window.enableSubmit = function() {
					var btn = document.getElementById("btnSubmit");
					btn.onclick = submit;
					btn.style.cursor = 'pointer';
					btn.style.backgroundColor = 'blue';
				}
				window.submit = function() {
					disableSubmit();
					var input = document.getElementById("usernameInput");
					var req = new XMLHttpRequest();
					req.onload = function() {
						if (req.status !== 200) {
							var error = document.getElementById("spnError");
							error.style.visibility = "visible";
							enableSubmit();
						} else {
							var success = document.getElementById("spnSuccess");
							success.style.visibility = "visible";
						}
					}
					req.onerror = function() {
						var error = document.getElementById("spnError");
						error.style.visibility = "visible";
						enableSubmit();
					}
					req.open("GET", "/pollbot/confirm?vote=%s&username=" + input.value);
					req.send();
				}
				window.onload = function() {
					var input = document.getElementById("usernameInput");
					input.addEventListener("keyup", function(event) {
						if (event.keyCode === 13) {
							event.preventDefault();
							document.getElementById("btnSubmit").click();
						}
					});
				}
				</script>
		</head>
		<body style="padding: 40px">
			<div style="display: flex; flex-direction: column; width: 600px; margin: auto; font-family: sans-serif;">
				<div style="display: flex">
					<span style="font-size: 24px; font-weight: 800; text-align: center; padding: 10px">Polling Service Anonymous Poll Confirmation</span>
					<div style="display: flex; flex-direction: column;">
						<span style="text-align: center; margin-bottom: 16px">Please enter your Keybase username below in order to confirm your selection of option (%d) in a recent poll in %s. </span>
						<span style="text-align: center">The Polling Service will contact you privately in the Keybase app in order to confirm the selection.</span>
					</div>
				</div>
				<div style="display: flex; flex-direction: column; width: 100%%; margin-top: 24px">
					<span>Username</span>
					<div style="display: flex; width: 100%%; margin-top: 4px; cursor: pointer;">
						<input id="usernameInput" style="width: 100%%; height: 40px; margin-right: 8px; font-size: 24px; padding: 8px" />
						<div id="btnSubmit" style="display: flex; background-color: blue; border-radius: 4px; align-items: center; justify-content: center; width: 100;" onclick="submit()">
							<span style="text-align: center; color: white">Submit</span>
						</div>
					</div>
					<span id="spnSuccess" style="visibility: hidden; color: green">Success!</span>
					<span id="spnError" style="visibility: hidden; color: red">Something went wrong.</span>
				</div>
			</div>
		</body>
	</html>
	`, vstr, vote.Choice, h.getConvName(conv))
	w.Write([]byte(body))
}

func (h *HTTPSrv) Listen() {
	http.HandleFunc("/pollbot/confirm", h.handleConfirm)
	http.HandleFunc("/pollbot/vote", h.handleVote)
	http.ListenAndServe(":8080", nil)
}
