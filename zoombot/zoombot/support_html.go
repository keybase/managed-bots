package zoombot

const supportHTML = `<html lang="en">
<title>
  Keybase Meeting Link Bot Support
</title>

<body>
  <h2 id="installation">Installation</h2>
  <ul>
    <li>Step by step guide<ol>
        <li>Install Keybase (<a href="https://keybase.io/download">https://keybase.io/download</a>)</li>
        <li>Add the Keybase Meeting Link Bot to your teams and conversations
        through the following steps:<ol>
            <li>Navigate to the conversation you wish to install Zoom Bot for</li>
            <li>Click the 'Chat info &amp; sessions' button (the icon is an i in
            a circle) in the top right corner</li>
            <li>Select the 'Bots' tab</li>
            <li>Scroll down to the entry that says 'Zoom Bot' and click the
            plus. Keybase will guide you through installing the bot</li>
          </ol>
        </li>
      </ol>
    </li>
    <li>Uninstalling or deauthorizing the Keybase Zoom Bot from your Zoom Account<ul>
        <li>Navigate to <a
            href="https://marketplace.zoom.us/user/installed">https://marketplace.zoom.us/user/installed</a> and click
          on
          the “Uninstall” button next to the Keybase integration</li>
      </ul>
    </li>
  </ul>
  <h2 id="usage">Usage</h2>
  <ul>
    <li><code>!zoom</code>: Create a new Zoom meeting<ul>
        <li>To use the bot, type <code>!zoom</code> in conversations that the
        Keybase Zoom Bot has been added to.  If you haven't used the bot before,
        it will direct message you in order to authenticate with Zoom.
        Otherwise, the bot will broadcast a Zoom Instant Meeting link on your
        behalf to the conversation that you sent the command into</li>
      </ul>
    </li>
  </ul>
  <h2 id="contact-support">Contact Support</h2>
  <ul>
    <li>Use the <code>!zoombot feedback &lt;feedback here&gt;</code> to provide feedback and questions.</li>
    <li>Report issues here: <a
        href="https://github.com/keybase/managed-bots/issues">https://github.com/keybase/managed-bots/issues</a></li>
    <li>Join @keybasefriends to discuss with the community <a
        href="https://keybase.io/team/keybasefriends">https://keybase.io/team/keybasefriends</a></li>
    <li>In app going to <code>Settings &gt; Feedback</code> to submit bugs or <code>keybase log send</code> from the
      command line.</li>
  </ul>
</body>

</html>
`
