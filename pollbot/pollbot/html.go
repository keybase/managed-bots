package pollbot

import "fmt"

const htmlVoteResult = `<html>
<head>
  <title>
	Polling Service Confirmation
  </title>
  <style>
  body {
	padding: 50px;
	font-family: 'Lucida Sans', 'Lucida Sans Regular', 'Lucida Grande', 'Lucida Sans Unicode', Geneva,
	  Verdana, sans-serif;
	font-size: 22px;
  }
  .column {
	display: flex;
	flex-direction: column;
  }
  .instructions {
	margin-top: 16px;
	text-align: center;
  }
  #divContainer {
	justify-content: center;
	align-items: center;
  }
  #imgBallot {
	width: 300px;
	height: 300px;
  }
  </style>
</head>
<body>
<div id="divContainer" class="column">
	<img src="/pollbot/image?=ballot" id="imgBallot" />
	<span style="font-size: 32px; margin-bottom: 24px; text-align: center;">%s</span>
</div>
</body>
</html>`

func makeHTMLVoteResult(result string) string {
	return fmt.Sprintf(htmlVoteResult, result)
}

const htmlLoginSuccess = `<html>
<head>
  <title>
	Polling Service Confirmation
  </title>
  <style>
  body {
	padding: 50px;
	font-family: 'Lucida Sans', 'Lucida Sans Regular', 'Lucida Grande', 'Lucida Sans Unicode', Geneva,
	  Verdana, sans-serif;
	font-size: 22px;
  }
  .column {
	display: flex;
	flex-direction: column;
  }
  .instructions {
	margin-top: 16px;
	text-align: center;
  }
  #divContainer {
	justify-content: center;
	align-items: center;
  }
  #imgBallot {
	width: 300px;
	height: 300px;
  }
  </style>
</head>
<body>
<div id="divContainer" class="column">
	<img src="/pollbot/image?=ballot" id="imgBallot" />
	<span style="font-size: 32px; margin-bottom: 24px; text-align: center;">Login success!</span>
	<span class="instructions">You can now vote in anonymous polls with a single click.</span>
</div>
</body>
</html>`

const htmlLogin = `<html>
<head>
  <title>
	Polling Service Login
  </title>
  <style>
	body {
	  padding: 50px;
	  font-family: 'Lucida Sans', 'Lucida Sans Regular', 'Lucida Grande', 'Lucida Sans Unicode', Geneva,
		Verdana, sans-serif;
	  font-size: 22px;
	}
	a {
	  color: black;
	}

	.row {
	  display: flex;
	  flex-direction: row;
	}
	.column {
	  display: flex;
	  flex-direction: column;
	}
	.instructions {
	  margin-top: 16px;
	  text-align: center;
	}
	.quote {
	  font-family: 'Courier New', Courier, monospace;
	  background-color: bisque;
	  color: blue;
	  margin-left: 2px;
	  margin-right: 2px;
	  border-radius: 2px;
	}

	#divLogin {
	  justify-content: center;
	  align-items: center;
	  width: 600px;
	  margin: auto;
	}
	#divContainer {
	  justify-content: center;
	  align-items: center;
	}
	#imgBallot {
	  width: 300px;
	  height: 300px;
	}
  </style>
</head>
<body>
  <div id="divContainer" class="column">
	<img src="/pollbot/image?=ballot" id="imgBallot" />
	<div id="divLogin" class="column">
	  <span style="font-size: 32px; margin-bottom: 24px; text-align: center;">Login Required</span>
	  <span class="instructions">
		In order to vote in anonymous polls, you must first login to the polling service in your web
		browser.
	  </span>
	  <span class="instructions">
		To start the login process, message <a target="_" href="https://keybase.io/pollbot">@pollbot</a> in the Keybase app with the text <span class="quote">login</span>.
	  </span>
	</div>
  </div>
</body>
</html>
`
