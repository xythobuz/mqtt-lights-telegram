# Lights-Telegram

Simple MQTT bridge in the form of a Telegram Bot.
And also my first attempt at doing something with Go.

## Configuration

First register a new Bot with the Telegram, as [described in the docs](https://core.telegram.org/bots#6-botfather), to get your API key.

Then simply run lights-telegram once manually using `go run .`.
This will create a `config.yaml` file in the same directory.
Your API key goes in there, as well as the MQTT credentials.

With the config prepared, run lights-telegram again.
Now send a message to the bot.
You will see the UserID of your Telegram account in the log output.
Quit lights-telegram again by pressing Ctrl+C, then open `config.yaml` again and put your User ID into the `admin_id` field.
Now this admin account can add further user authorizations using chat messages to the bot.

As an admin use `/register` to add new MQTT topics and their possible values to the menu.
Finally run `/commandlist` on the bot to get a nicely formatted command list which you can then give to the `/setcommands` command of BotFather.

## Installation

Run `go build` to produce an executable in the project directory.

You now need to decide on an installation location for both the executable as well as the configuration file.
As an example, I'll be using `~/bin/lights-telegram/` as the install directory.

You also need to decide which user will run the server.
It needs write access to the directory / config file.
In this example we'll also be using the current user, `thomas` in my case.

Modify the username and paths in `lights-telegram.service` and the commands below as required.

    CGO_ENABLED=0 go build
    mkdir ~/bin/lights-telegram
    cp lights-telegram ~/bin/lights-telegram/lights-telegram
    cp config.yaml ~/bin/lights-telegram/config.yaml
    cp lights-telegram.service /etc/systemd/system/lights-telegram.service
    systemctl daemon-reload
    systemctl enable --now lights-telegram

You can also look at the log output of the app.

    journalctl -u lights-telegram -b -f

## Dependencies

 * [tgbotapi](https://pkg.go.dev/github.com/go-telegram-bot-api/telegram-bot-api/v5)
 * [mqtt](https://pkg.go.dev/github.com/eclipse/paho.mqtt.golang)
 * [yaml](https://pkg.go.dev/gopkg.in/yaml.v3)

## License

    ----------------------------------------------------------------------------
    "THE BEER-WARE LICENSE" (Revision 42):
    <xythobuz@xythobuz.de> wrote this file.  As long as you retain this notice
    you can do whatever you want with this stuff. If we meet some day, and you
    think this stuff is worth it, you can buy me a beer in return.   Thomas Buck
    ----------------------------------------------------------------------------
