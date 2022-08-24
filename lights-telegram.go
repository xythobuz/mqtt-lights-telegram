package main

import (
    "fmt"
    "log"
    "os"
    "strings"
    "strconv"
    "errors"
    "io/ioutil"
    "gopkg.in/yaml.v3"
    "github.com/eclipse/paho.mqtt.golang"
    tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var configFilename = "config.yaml"

type Mqtt struct {
    Url string `yaml:"url"`
    User string `yaml:"username"`
    Pass string `yaml:"password"`
}

type Registration struct {
    Name string `yaml:"name"`
    Topic string `yaml:"topic"`
    Values []string `yaml:"values"`
    lastValue string
}

type Config struct {
    // Telegram Bot API key
    Key string `yaml:"api_key"`

    // Telegram UserID (int64) of admin account
    Admin int64 `yaml:"admin_id"`

    // MQTT credentials
    Mqtt Mqtt

    // Telegram UserIDs (int64) of allowed users
    // (does not need to be modified manually)
    Users []int64 `yaml:"authorized_users"`

    // Available MQTT topics
    // (does not need to be modified manually)
    Registration []Registration
}

// default values
var config = Config {
    Key: "API_KEY_GOES_HERE",
    Admin: 0,
    Mqtt: Mqtt {
        Url: "wss://MQTT_HOST:MQTT_PORT",
        User: "MQTT_USERNAME",
        Pass: "MQTT_PASSWORD",
    },
}

var bot *tgbotapi.BotAPI = nil
var mqttClient mqtt.Client = nil

func readConfig() error {
    // read config file
    file, err := ioutil.ReadFile(configFilename)
    if err != nil {
        log.Printf("Conf file error: %v", err)
        return err
    }

    // parse yaml into struct
    err = yaml.Unmarshal(file, &config)
    if err != nil {
        log.Printf("Conf yaml error: %v", err)
        return err
    }

    return nil
}

func writeConfig() error {
    // parse struct into yaml
    data, err := yaml.Marshal(config)
    if err != nil {
        log.Printf("Conf yaml error: %v", err)
        return err
    }

    // write config file
    err = ioutil.WriteFile(configFilename, data, 0644)
    if err != nil {
        log.Printf("Conf file error: %v", err)
        return err
    }

    return nil
}

func isAdmin(id int64) bool {
    if id == config.Admin {
        return true
    }

    return false
}

func isAuthorizedUser(id int64) bool {
    if isAdmin(id) {
        return true
    }

    for user := range config.Users {
        if id == config.Users[user] {
            return true
        }
    }

    return false
}

func addAuthorizedUser(id int64) error {
    if isAdmin(id) {
        // admin is always authorized
        return nil
    }

    for user := range config.Users {
        if id == config.Users[user] {
            // already in users list
            return nil
        }
    }

    config.Users = append(config.Users, id)
    return writeConfig()
}

func sendReply(text string, chat int64, message int) {
    msg := tgbotapi.NewMessage(chat, text)
    msg.ReplyToMessageID = message
    msg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
    _, err := bot.Send(msg)
    if err != nil {
        log.Printf("Bot error: %v", err)
    }
}

func sendMessage(text string, user int64) {
    // UserID == ChatID
    msg := tgbotapi.NewMessage(user, text)
    msg.ReplyMarkup = tgbotapi.NewRemoveKeyboard(true)
    _, err := bot.Send(msg)
    if err != nil {
        log.Printf("Bot error: %v", err)
    }
}

func sendKeyboardReply(text string, name string, chat int64, message int) {
    var rows [][]tgbotapi.KeyboardButton
    for reg := range config.Registration {
        if name == config.Registration[reg].Name {
            for value := range config.Registration[reg].Values {
                button := tgbotapi.NewKeyboardButton("/" + name + " " + config.Registration[reg].Values[value])
                row := tgbotapi.NewKeyboardButtonRow(button)
                rows = append(rows, row)
            }
        }
    }
    keyboard := tgbotapi.NewOneTimeReplyKeyboard(rows...)

    msg := tgbotapi.NewMessage(chat, text)
    msg.ReplyToMessageID = message
    msg.ReplyMarkup = keyboard
    _, err := bot.Send(msg)
    if err != nil {
        log.Printf("Bot error: %v", err)
    }
}

func sendGenericKeyboardReply(text string, chat int64, message int) {
    var rows [][]tgbotapi.KeyboardButton
    //var rows []tgbotapi.KeyboardButton
    for reg := range config.Registration {
        button := tgbotapi.NewKeyboardButton("/" + config.Registration[reg].Name)
        row := tgbotapi.NewKeyboardButtonRow(button)
        //row := button
        rows = append(rows, row)
    }
    keyboard := tgbotapi.NewOneTimeReplyKeyboard(rows...)
    //keyboard := tgbotapi.NewOneTimeReplyKeyboard(rows)

    msg := tgbotapi.NewMessage(chat, text)
    msg.ReplyToMessageID = message
    msg.ReplyMarkup = keyboard
    _, err := bot.Send(msg)
    if err != nil {
        log.Printf("Bot error: %v", err)
    }
}

func notifyAdminAuthorization(id int64, name string) {
    if (config.Admin == 0) {
        // probably no admin account configured yet. don't ask them.
        return
    }

    log.Printf("Requesting admin authorization for new user %s.", name)
    text := fmt.Sprintf("New connection from %s. Send \"/auth %d\" to authorize.", name, id)
    sendMessage(text, config.Admin)
}

func sendMqttMessage(topic string, msg string) {
    log.Printf("MQTT Tx: %s @ %s", msg, topic)
    token := mqttClient.Publish(topic, 0, true, msg)
    token.Wait()
}

func register(name string, topic string, values string) error {
    for reg := range config.Registration {
        if name == config.Registration[reg].Name {
            return errors.New("already registered")
        }
    }

    v := strings.Split(values, ",")
    r := Registration {
        Name: name,
        Topic: topic,
        Values: v,
    }

    config.Registration = append(config.Registration, r)
    writeConfig()


    token := mqttClient.Subscribe(topic, 0, onMessageReceived)
    if token.Wait() && token.Error() != nil {
        log.Printf("MQTT sub error: %v", token.Error())
    }

    return nil
}

func remove(s []Registration, i int) []Registration {
    s[i] = s[len(s) - 1]
    return s[:len(s) - 1]
}

func unregister(name string) error {
    for reg := range config.Registration {
        if name == config.Registration[reg].Name {
            token := mqttClient.Unsubscribe(config.Registration[reg].Topic)
            if token.Wait() && token.Error() != nil {
                log.Println("MQTT unsub error: %v", token.Error())
            }

            config.Registration = remove(config.Registration, reg)
            writeConfig()

            return nil
        }
    }

    return errors.New("name not found")
}

func isRegisteredCommand(name string) bool {
    for reg := range config.Registration {
        if name == config.Registration[reg].Name {
            return true
        }
    }
    return false
}

func isValidValue(name string, val string) bool {
    for reg := range config.Registration {
        if name == config.Registration[reg].Name {
            for value := range config.Registration[reg].Values {
                if val == config.Registration[reg].Values[value] {
                    return true
                }
            }
        }
    }
    return false
}

func topicForName(name string) string {
    for reg := range config.Registration {
        if name == config.Registration[reg].Name {
            return config.Registration[reg].Topic
        }
    }
    return "unknown"
}

func lastValueForCommand(name string) string {
    ret := ""

    for reg := range config.Registration {
        if name == config.Registration[reg].Name {
            if len(config.Registration[reg].lastValue) > 0 {
                ret = "Current state: \""
                ret += config.Registration[reg].lastValue
                ret += "\"\n"
            }
            break
        }
    }

    ret += "Select option below..."
    return ret
}

func onMessageReceived(client mqtt.Client, message mqtt.Message) {
    log.Printf("MQTT Rx: %s @ %s", message.Payload(), message.Topic())

    for reg := range config.Registration {
        if config.Registration[reg].Topic == message.Topic() {
            config.Registration[reg].lastValue = string(message.Payload()[:])
        }
    }
}

func main() {
    err := readConfig()
    if err != nil {
        log.Printf("Can't read config file \"%s\".", configFilename)
        log.Printf("Writing default values. Please modify.")
        writeConfig()
        os.Exit(1)
    }

    // MQTT debugging
    //mqtt.ERROR = log.New(os.Stdout, "[ERROR] ", 0)
    //mqtt.CRITICAL = log.New(os.Stdout, "[CRIT] ", 0)
    //mqtt.WARN = log.New(os.Stdout, "[WARN]  ", 0)
    //mqtt.DEBUG = log.New(os.Stdout, "[DEBUG] ", 0)

    // Initialize MQTT
    opts := mqtt.NewClientOptions()
    opts.AddBroker(config.Mqtt.Url)
    opts.SetClientID("lights-telegram")
    opts.SetUsername(config.Mqtt.User)
    opts.SetPassword(config.Mqtt.Pass)

    mqttClient = mqtt.NewClient(opts)
    token := mqttClient.Connect();
    if token.Wait() && token.Error() != nil {
        log.Printf("MQTT error: %v", token.Error())
        os.Exit(1)
    }

    // Subscribe to registered topics
    for reg := range config.Registration {
        token := mqttClient.Subscribe(config.Registration[reg].Topic, 0, onMessageReceived)
        if token.Wait() && token.Error() != nil {
            log.Printf("MQTT sub error: %v", token.Error())
        }
    }

    // Initialize Telegram
    bot, err = tgbotapi.NewBotAPI(config.Key)
    if err != nil {
        log.Fatalf("Bot error: %v", err)
    }

    // Telegram debugging
    //bot.Debug = true

    // Start message receiving
    log.Printf("Authorized on account %s", bot.Self.UserName)
    u := tgbotapi.NewUpdate(0)
    u.Timeout = 60
    updates := bot.GetUpdatesChan(u)
    for update := range updates {
        if update.Message == nil {
            continue
        }

        log.Printf("[Rx \"%s\"] %s", update.Message.From.UserName, update.Message.Text)

        reply := ""
        showGenericKeyboard := false

        if isAuthorizedUser(update.Message.From.ID) {
            switch {
                case update.Message.Text == "/start":
                    reply = "Welcome to the Lights control bot! Try /help for tips."
                    showGenericKeyboard = true

                case update.Message.Text == "/help":
                    if len(config.Registration) > 0 {
                        reply += "You can use the following commands:\n"
                        for reg := range config.Registration {
                            reply += fmt.Sprintf(" - /%s", config.Registration[reg].Name)
                            for val := range config.Registration[reg].Values {
                                reply += fmt.Sprintf(" %s", config.Registration[reg].Values[val])
                            }
                            reply += "\n"
                        }
                        reply += "\n"
                    }

                    reply += "These commands are always available:\n"
                    reply += " - /send TOPIC VALUE\n"
                    reply += " - /help\n"
                    reply += " - /start\n"

                    if isAdmin(update.Message.From.ID) {
                        reply += "\nYou are an administrator, so you can also use:\n"
                        reply += " - /auth ID\n"
                        reply += " - /register NAME TOPIC VAL1,VAL2,...\n"
                        reply += " - /unregister NAME\n"
                        reply += " - /commandlist"
                    } else {
                        reply += "\nAdministrators have further options not available to you."
                    }

                    showGenericKeyboard = true

                case strings.HasPrefix(update.Message.Text, "/auth "):
                    if isAdmin(update.Message.From.ID) {
                        id, err := strconv.ParseInt(update.Message.Text[6:], 10, 64)
                        if err != nil {
                            reply = fmt.Sprintf("Error parsing ID! %v", err)
                        } else {
                            err = addAuthorizedUser(id)
                            if err != nil {
                                reply = fmt.Sprintf("Error authorizing ID! %v", err)
                            } else {
                                reply = fmt.Sprintf("Ok, authorized %d.", id)

                                // also notify user
                                text := "You have now been authorized by the admin. Try /help for commands."
                                sendMessage(text, id)
                            }
                        }
                    } else {
                        reply = "Sorry, only administrators can do that!"
                    }

                case strings.HasPrefix(update.Message.Text, "/send "):
                    s := update.Message.Text[6:]
                    topic, msg, found := strings.Cut(s, " ")
                    if found {
                        reply = fmt.Sprintf("Setting \"%s\" to \"%s\"", topic, msg)
                        sendMqttMessage(topic, msg)
                    } else {
                        reply = "Error parsing your message."
                    }

                case strings.HasPrefix(update.Message.Text, "/register "):
                    if isAdmin(update.Message.From.ID) {
                        s := update.Message.Text[10:]
                        name, rest, found := strings.Cut(s, " ")
                        if found {
                            topic, values, found := strings.Cut(rest, " ")
                            if found {
                                err = register(name, topic, values)
                                if err != nil {
                                    reply = fmt.Sprintf("Error registering! %v", err)
                                } else {
                                    reply = fmt.Sprintf("Ok, registered %s", name)
                                }
                            } else {
                                reply = fmt.Sprintf("Error parsing topic!")
                            }
                        } else {
                            reply = fmt.Sprintf("Error parsing name!")
                        }
                    } else {
                        reply = "Sorry, only administrators can do that!"
                    }

                case strings.HasPrefix(update.Message.Text, "/unregister "):
                    if isAdmin(update.Message.From.ID) {
                        name := update.Message.Text[12:]
                        err = unregister(name)
                        if err != nil {
                            reply = fmt.Sprintf("Error unregistering! %v", err)
                        } else {
                            reply = fmt.Sprintf("Ok, unregistered %s", name)
                        }
                    } else {
                        reply = "Sorry, only administrators can do that!"
                    }

                case update.Message.Text == "/commandlist":
                    if isAdmin(update.Message.From.ID) {
                        for reg := range config.Registration {
                            reply += fmt.Sprintf("%s - Set '%s' state\n", config.Registration[reg].Name, config.Registration[reg].Topic)
                        }
                        reply += "help - Show help text and keyboard"
                    } else {
                        reply = "Sorry, only administrators can do that!"
                    }

                default:
                    reply = "Sorry, I did not understand. Try /help instead."
                    name, val, found := strings.Cut(update.Message.Text, " ")
                    name = name[1:] // remove '/'
                    if found {
                        if isRegisteredCommand(name) {
                            if isValidValue(name, val) {
                                sendMqttMessage(topicForName(name), val)
                                reply = fmt.Sprintf("Ok, setting %s to %s", name, val)
                            } else {
                                reply = "Sorry, this is not a valid value! Try /help instead."
                            }
                        } else {
                            reply = "Sorry, this command is not registered. Try /help instead."
                        }
                    } else if isRegisteredCommand(update.Message.Text[1:]) {
                        reply = ""
                        text := lastValueForCommand(update.Message.Text[1:])
                        sendKeyboardReply(text, update.Message.Text[1:], update.Message.Chat.ID, update.Message.MessageID)
                    }
            }
        } else {
            // only request admin-auth when /start has been sent!
            // this avoids most bot spam.
            if update.Message.Text == "/start" {
                log.Printf("Message from unauthorized user. %s %d", update.Message.From.UserName, update.Message.From.ID)
                notifyAdminAuthorization(update.Message.From.ID, update.Message.From.UserName)

                reply = "Sorry, you are not authorized. Administrator confirmation required."
            }
        }

        // send a reply
        if reply != "" {
            log.Printf("[Tx \"%s\"] %s", update.Message.From.UserName, reply)

            if showGenericKeyboard {
                sendGenericKeyboardReply(reply, update.Message.Chat.ID, update.Message.MessageID)
            } else {
                sendReply(reply, update.Message.Chat.ID, update.Message.MessageID)
            }
        }
    }
}
