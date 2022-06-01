# cfNotificationService
This tool allows users to subscribe to notifications that are of importance to them as cloudfoundry platform users. The cfServiceAlert tool uses this service to get the alerts to the end-users. But since this is a simple web service you can also use it to send alerts from scripts, pipelines, and so on.

## deploying
This tool is intended to run on CF. Use the provided manifest.yml to deploy it. Please check config.go for the environment variables that are expected. Since this still very much in development I havent taken the time to document all env vars. 

## Using - sending messages
HTTP PUT to <url>/send with json body that looks like this:
```
{
  "id": "<ID for this message>",
  "subject": "<subject line>",
  "message": "<message body>",
  "validity": "<How long will the message be kept. If a message with the exact same ID is sent wihtin this time it won't be forwarded to users>",
  "target": {
      "type": "<space or idmgroup>",
      "environment": "<must match one of the environment configured through env vars>",
      "Id": "<name of the entity you're addressing. So if type is set to "space" then this will be the space name you're targetting>"
  }
}
```
## using - receiving messages
Users of this service can subscribe to this service by simply logging in with their CF account and then entering and saving the address on which they would like to recieve messages. 
Once a user is subscribe he will receive message for the CF spaces or idb groups he is a member of. 
