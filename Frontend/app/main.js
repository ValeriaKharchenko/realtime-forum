let ws = null;
let chat = document.getElementById("chat-messages");
let receiver = document.getElementById("receiver");
let msgField = document.getElementById("message");

export const WS = () => {
    window.onbeforeunload = function () {
        console.log("Leaving...");
        let jsonData = {};
        jsonData['action'] = 'left';
        ws.send(JSON.stringify(jsonData));
    }
    document.addEventListener("DOMContentLoaded", function () {
        ws = new WebSocket('ws://' + window.location.host + '/ws');
        ws.onopen = () => {
            console.log("Successfully connected");
        }
        ws.onclose = () => {
            console.log("connection closed");
        }
        ws.onerror = error => {
            console.log("error occurred", error);
        }
        ws.onmessage = msg => {
            let data = JSON.parse(msg.data);
            console.log("Action is ", data.action);
            switch (data.action) {
                case "list_users":
                    let list = document.getElementById('online-users');
                    while (list.firstChild) list.removeChild(list.firstChild);
                    if (data.connected_users.length > 0) {
                        data.connected_users.forEach(function (item) {
                            console.log(item);
                            let li = document.createElement("li");
                            li.appendChild(document.createTextNode(item));
                            list.appendChild(li);
                        });
                    }
                    break;

                case "broadcast":
                    console.log(data.message);
                    chat.innerHTML = chat.innerHTML + data.message + "<br>";
                    break;
            }
        }

        // receiver.addEventListener('change', function () {
        //     let jsonData = {};
        //     jsonData['action'] = 'username';
        //     jsonData['user_name'] = this.value;
        //     ws.send(JSON.stringify(jsonData));
        // });

        msgField.addEventListener('keydown', (e) => {
            if (e.code === 'Enter') {
                if (!ws) {
                    console.log("no connection");
                    return false;
                }
                if (receiver.value === "" || msgField.value === "") {
                    alert("fill out user and message fields");
                    return false;
                } else {
                    sendMessage();
                }
                    e.preventDefault();
                    e.stopPropagation();
                }
        })
        document.getElementById("send-button").addEventListener('click', () => {
            if (receiver.value === "" || msgField.value === "") {
                alert("fill out user and message fields");
                return false;
            } else {
                sendMessage();
            }
        });
    });
}

function sendMessage() {
    let jsonData = {};
    jsonData['action'] = 'broadcast';
    jsonData['receiver'] = receiver.value;
    jsonData['message'] = msgField.value;
    ws.send(JSON.stringify(jsonData));
    msgField.value = "";
}