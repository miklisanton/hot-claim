# HOT-CLAIM
Go application to automatically claim hot tokens with/without proxy
## Usage
You need to install docker and create config.json file

`
git clone https://github.com/miklisanton/hot-claim.git
cd hot-claim
touch config.json
docker run -d $(docker build -q .)
`
### config.json format
First, you need to claim manually and capture headers sent by claim or status methods using f12->network
To avoid using proxy, set "proxy" to ""

```json
{
  "accounts": [
    {
      "device_id": "",
      "authorization": "",
      "telegram_data": "",
      "user_agent": "",
      "proxy": "http://login:password@host:port",
      "username": "username.tg"
    }
  ]
}
```