#Pesatu - a Webchat and Videocall app in golang

Introducing Pesatu - This is a simple webchat application that uses Golang, Websocket, and WebRTC technologies to provide real-time communication between users.

## Installation Backend API

To install the application, you need to follow the steps below:

1. Clone the repository:
```
git clone https://github.com/royyanwibisono/Pesatu.git
```

2. Navigate to the root directory and install the required dependencies:
```
go mod init
```

3. Edit .env from env_example.txt configuration

## Usage

To run the application, navigate to the root directory and execute the following command:
```
go run main.go -a=:7000 -dev=2 -env=.env
```

## Installation Frontend app (ReactJS)

1. Clone frontend repository on differrent folder, make sure you have node in your machine:
```
https://github.com/royyanwibisono/oj-ion-app.git
```
2. Follow insruction on readme

## Credits

- Gin framework: https://github.com/gin-gonic/gin
- Gorilla WebSocket package: https://github.com/gorilla/websocket
- WEBRTC SFU : https://github.com/ionorg/ion-sfu
