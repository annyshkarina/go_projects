package main

import (
	"log"
	"os"
        "strings"
        "path/filepath"
        "database/sql"
        _ "github.com/lib/pq"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

var db *sql.DB
var botToken = "8123646360:AAFiDjEtsvY7v9_CNGckPmnQzdkM5N0Beio"

func initDB() {
    var err error
    connStr := "host=Localhost port=5432 user=zapovednik password=1234 dbname=zapovedniks sslmode=disable"
    db, err = sql.Open("postgres", connStr)
    if err != nil {
        log.Fatal("Ошибка подключения к базе данных:", err)
    }
    if err := db.Ping(); err != nil {
        log.Fatal("База данных недоступна:", err)
    }
}


func main() {
        initDB()
	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Panic(err)
	}

	bot.Debug = true
	log.Printf("Авторизованный на сервере аккаунт %s", bot.Self.UserName)

	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 60
	updates := bot.GetUpdatesChan(updateConfig)

	for update := range updates {
		if update.Message != nil {
			handleMessage(bot, update.Message)
		} else if update.CallbackQuery != nil {
			handleCallback(bot, update.CallbackQuery)
		}
	}
}

func sendPhotosByPlaceID(bot *tgbotapi.BotAPI, chatID int64, placeID int) {
    rows, err := db.Query("SELECT filepath FROM photos WHERE place_id = $1", placeID)
    if err != nil {
        log.Println("Ошибка запроса к БД:", err)
        return
    }
    defer rows.Close()

    log.Println("Выполняется запрос фото для place_id:", placeID)

    var mediaGroup []interface{}
    count := 0
    for rows.Next() {
        count ++
        var path string
        err := rows.Scan(&path)
        if err != nil {
            log.Println("Ошибка чтения пути фото из БД:", err)
            continue
        }
        log.Println("Найден путь к фото:", path)
        fullpath := "/Users/annaskarina/go_projects/" + filepath.Base(path)
        
        if _, err := os.Stat(fullpath); err == nil {
             media := tgbotapi.NewInputMediaPhoto(tgbotapi.FilePath(fullpath))
             mediaGroup = append(mediaGroup, media)
        } else {
             log.Println("Файл не найден:", fullpath)
        continue
        }
    }
      
    log.Println("Всего найдено строк:", count)

    if len(mediaGroup) > 0 {
        mediaConfig := tgbotapi.MediaGroupConfig{
            ChatID: chatID,
            Media:  mediaGroup,
        }
        _, err := bot.SendMediaGroup(mediaConfig)
        if err != nil {
                 log.Println("Ошибка отправки фото", err)
        }
    } else {
        bot.Send(tgbotapi.NewMessage(chatID, "Фотографии не найдены."))
    }
}

func sendPlaceDescription(bot *tgbotapi.BotAPI, chatID int64, placeID int) {
    var description string
    err := db.QueryRow("SELECT description FROM places WHERE id = $1", placeID).Scan(&description)
    if err != nil {
        log.Println("Ошибка получения описания:", err)
        bot.Send(tgbotapi.NewMessage(chatID, "Описание не найдено."))
        return
    }

    msg := tgbotapi.NewMessage(chatID, description)
    bot.Send(msg)
}

func sendPlaceSectionSplit(bot *tgbotapi.BotAPI, chatID int64, placeID int, section string) {
    var info string
    var descQuery, photoQuery string

    switch section {
    case "flora":
        descQuery = "SELECT flora_info FROM places WHERE id = $1"
        photoQuery = "SELECT flora_foto FROM photos WHERE place_id = $1 AND flora_foto IS NOT NULL"
    case "fauna":
        descQuery = "SELECT fauna_info FROM places WHERE id = $1"
        photoQuery = "SELECT fauna_foto FROM photos WHERE place_id = $1 AND fauna_foto IS NOT NULL"
    default:
        bot.Send(tgbotapi.NewMessage(chatID, "Раздел не найден."))
        return
    }

    // Получаем описание
    err := db.QueryRow(descQuery, placeID).Scan(&info)
    if err != nil || info == "" {
        log.Println("Описание не найдено:", section, err)
        bot.Send(tgbotapi.NewMessage(chatID, "Описание отсутствует."))
        return
    }

    title := map[string]string{
        "flora": "🌿 Флора",
        "fauna": "🦌 Фауна",
    }[section]

    // Отправляем сначала текст
    _, err = bot.Send(tgbotapi.NewMessage(chatID, title+"\n\n"+info))
    if err != nil {
        log.Println("Ошибка отправки текста:", err)
    }

    // Получаем фото
    rows, err := db.Query(photoQuery, placeID)
    if err != nil {
        log.Println("Ошибка запроса фото:", err)
        return
    }
    defer rows.Close()

    var mediaGroup []interface{}
    for rows.Next() {
        var path string
        if err := rows.Scan(&path); err != nil {
            log.Println("Ошибка чтения пути фото:", err)
            continue
        }
        log.Println("Найден путь к фото:", path)
        fullpath := "/Users/annaskarina/go_projects/" + filepath.Base(path)

        if _, err := os.Stat(fullpath); err == nil {
             media := tgbotapi.NewInputMediaPhoto(tgbotapi.FilePath(fullpath))
             mediaGroup = append(mediaGroup, media)
        } else {
             log.Println("Файл не найден:", fullpath)
        continue
        }
    }
    

    // Отправляем фото как медиагруппу, если есть
    if len(mediaGroup) > 0 {
        _, err := bot.SendMediaGroup(tgbotapi.MediaGroupConfig{
            ChatID: chatID,
            Media:  mediaGroup,
        })
        if err != nil {
            log.Println("Ошибка отправки фото:", err)
        }
    } else {
        log.Println("Нет фото по разделу:", section)
    }
}



func handleMessage(bot *tgbotapi.BotAPI, message *tgbotapi.Message) {
        lowerText := strings.ToLower(message.Text)
	switch message.Text {
	case "/start":
		menu := tgbotapi.NewMessage(message.Chat.ID, "Здравствуйте! Я — ваш гид по природным богатствам Нижегородской области 🌿🌳. \nЗдесь вы можете узнать о заповедниках, парках и редких видах животных и растений 🦋🦉. \n🌍Выберите объект, который вас интересует!")
		menu.ReplyMarkup = mainMenu()
		bot.Send(menu)
	case "🌿 Керженский заповедник":
		sendInlineMenu(bot, message.Chat.ID, kerzhMenu())
	case "🌳 Природный парк 'Воскресенское Поветлужье'":
		sendInlineMenu(bot, message.Chat.ID, voskrMenu())
	case "🏚 Музей - заповедник 'Щелоковский хутор'":
		sendInlineMenu(bot, message.Chat.ID, selokovMenu())
        case "🏞 Ичалковский бор-заказник":
                sendInlineMenu(bot, message.Chat.ID, ichalkiMenu())
        case "🏖 Ботанический заказник 'Пустынский'":
                sendInlineMenu(bot, message.Chat.ID, pustynMenu())
        case "🚲 Заповедник 'Сергачский дендропарк - Явлейка'":
                sendInlineMenu(bot, message.Chat.ID, sergachMenu())
        case "🌾 Мухтоловский заказник":
                sendInlineMenu(bot, message.Chat.ID, muxMenu())
        case "🍂 Урочище Слуда":
                sendInlineMenu(bot, message.Chat.ID, sludaMenu())
        case "🌲 Стригинский бор":
                sendInlineMenu(bot, message.Chat.ID, striginoMenu())
        case "🔍 Поиск по ключевым словам":
                msg := tgbotapi.NewMessage(message.Chat.ID, "Введите ключевое слово, чтобы найти информацию:\n\n" + "- экскурсия\n" + "- флора/фауна\n" + "- вело\n" + "- лыжи\n" + "- детская площадка\n" + "- экотропы\n" + "- базы отдыха\n" + "- конный клуб\n" + "- пляж\n" + "- кафе\n\n" + "Бот подскажет, где это доступно.")
                bot.Send(msg)
        default:

	switch {
	case strings.Contains(lowerText, "щелоков"):
		sendInlineMenu(bot, message.Chat.ID, selokovMenu())
		return
        case strings.Contains(lowerText, "щёлоков"):
                sendInlineMenu(bot, message.Chat.ID, selokovMenu())
                return
        case strings.Contains(lowerText, "хутор"):
                sendInlineMenu(bot, message.Chat.ID, selokovMenu())
                return
	case strings.Contains(lowerText, "кержен"):
		sendInlineMenu(bot, message.Chat.ID, kerzhMenu())
		return
	case strings.Contains(lowerText, "воскресен"):
		sendInlineMenu(bot, message.Chat.ID, voskrMenu())
		return
	case strings.Contains(lowerText, "поветлуж"):
                sendInlineMenu(bot, message.Chat.ID, voskrMenu())
                return
        case strings.Contains(lowerText, "ичалк"):
                sendInlineMenu(bot, message.Chat.ID, ichalkiMenu())
                return
        case strings.Contains(lowerText, "пустын"):
                sendInlineMenu(bot, message.Chat.ID, pustynMenu())
                return
        case strings.Contains(lowerText, "сергач"):
                sendInlineMenu(bot, message.Chat.ID, sergachMenu())
                return
        case strings.Contains(lowerText, "дендр"):
                sendInlineMenu(bot, message.Chat.ID, sergachMenu())
                return
        case strings.Contains(lowerText, "явлейк"):
                sendInlineMenu(bot, message.Chat.ID, sergachMenu())
                return
        case strings.Contains(lowerText, "мухтолов"):
                sendInlineMenu(bot, message.Chat.ID, muxMenu())
                return
        case strings.Contains(lowerText, "слуда"):
                sendInlineMenu(bot, message.Chat.ID, sludaMenu())
                return
        case strings.Contains(lowerText, "урочище"):
                sendInlineMenu(bot, message.Chat.ID, sludaMenu())
                return
        case strings.Contains(lowerText, "стригин"):
                sendInlineMenu(bot, message.Chat.ID, striginoMenu())
                return
        case strings.Contains(lowerText, "экскурс"):
                msg := tgbotapi.NewMessage(message.Chat.ID, "Экскурсии доступны в следующих местах:")
                keyboard := tgbotapi.NewInlineKeyboardMarkup(
                        []tgbotapi.InlineKeyboardButton{
                                tgbotapi.NewInlineKeyboardButtonURL("🖋 Запись на экскурсию в Керженский заповедник", "http://www.kerzhenskiy.ru/osnovnye-napravleniya-deyatelnosti/ekoturizm/zayavka-na-ekskursiyu/"),
                        },
                        []tgbotapi.InlineKeyboardButton{
                                tgbotapi.NewInlineKeyboardButtonURL("🖋 Запись на экскурсию в Щелоковский хутор", "https://hutormuzey.ru/custom/9"),
                        },
                        []tgbotapi.InlineKeyboardButton{
                                tgbotapi.NewInlineKeyboardButtonURL("🖋 Запись на экскурсию в Ичалковский бор", "https://nn.kassir.ru/tourist/avtorskaya-ekskursiya-tur-v-ichalkovskie-pescheryi"),
                        },
                        []tgbotapi.InlineKeyboardButton{
                                tgbotapi.NewInlineKeyboardButtonURL("🖋 Запись на экскурсию в Воскресенское поветлужье", "https://vizit-povetluzhie.ru/excursions"),
                        },

                )
                msg.ReplyMarkup = keyboard
                bot.Send(msg)
                return
        case strings.Contains(lowerText, "троп"):
                msg := tgbotapi.NewMessage(message.Chat.ID, "Экотропы доступны в следующих местах:")
                keyboard := tgbotapi.NewInlineKeyboardMarkup(
                        []tgbotapi.InlineKeyboardButton{
                                 tgbotapi.NewInlineKeyboardButtonURL("🚶Экотропы в Воскресенском поветлужье", "https://vizit-povetluzhie.ru/ecotrails"),
                        },
                        []tgbotapi.InlineKeyboardButton{
                                 tgbotapi.NewInlineKeyboardButtonURL("🌿 Экотропы в Керженском заповеднике", "http://www.kerzhenskiy.ru/osnovnye-napravleniya-deyatelnosti/ekoturizm/opisanie-ekskursiy/"),
                        },
                )
                msg.ReplyMarkup = keyboard
                bot.Send(msg)
                return
        case strings.Contains(lowerText, "вело"):
                msg := tgbotapi.NewMessage(message.Chat.ID, "🚲 На велосипедах можно покататься во всех представленных заповедниках. Но специально оборудованные велодорожки имеются только в:")
                keyboard := tgbotapi.NewInlineKeyboardMarkup(
                        []tgbotapi.InlineKeyboardButton{
                                 tgbotapi.NewInlineKeyboardButtonURL("🏚 'Щелоковский хутор', протяженность маршрута 11,5 км", "https://yandex.com/maps/47/nizhny-novgorod/?from=mapframe&ll=44.014101%2C56.275212&mode=routes&rtext=56.280945%2C43.997531~56.276007%2C44.013395~56.279802%2C44.018301~56.286981%2C44.008909~56.288840%2C44.016694~56.288973%2C44.023477~56.286901%2C44.027919~56.283660%2C44.029065~56.280100%2C44.022474~56.276295%2C44.020243~56.273186%2C44.019814~56.273133%2C44.015849~56.269758%2C44.002237~56.271831%2C44.003287&rtt=bc&ruri=~ymapsbm1%3A%2F%2Forg%3Foid%3D193291983964~~~~~~~~~~~~&z=15.74"),
                        },
                       []tgbotapi.InlineKeyboardButton{
                                 tgbotapi.NewInlineKeyboardButtonURL("🌲 Стригинский бор, протяженность маршрута 5,2 км", "https://yandex.com/maps/47/nizhny-novgorod/?from=mapframe&ll=43.786764%2C56.195679&mode=routes&rtext=56.199667%2C43.798589~56.199186%2C43.787370~56.196683%2C43.772573~56.194082%2C43.785374~56.192458%2C43.786466~56.194096%2C43.800368~56.199608%2C43.798792&rtt=bc&ruri=~~~~~~&z=15"),
                        }, 
                )
                msg.ReplyMarkup = keyboard
                bot.Send(msg)
                return
        case strings.Contains(lowerText, "дет"), strings.Contains(lowerText, "площадк"), strings.Contains(lowerText, "игров"):
                msg := tgbotapi.NewMessage(message.Chat.ID, "Детские игровые площадки имеются в следующих местах:")
                keyboard := tgbotapi.NewInlineKeyboardMarkup(
                        []tgbotapi.InlineKeyboardButton{
                                 tgbotapi.NewInlineKeyboardButtonURL("🏚 Музей - заповедник 'Щелоковский хутор'", "https://yandex.ru/maps/org/detskaya_ploshchadka/232274322403/?ll=44.012537%2C56.271974&z=15.25"),
                        },
                        []tgbotapi.InlineKeyboardButton{
                                 tgbotapi.NewInlineKeyboardButtonURL("🏖 Ботанический заказник 'Пустынский'", "https://yandex.ru/maps/?display-text=%D0%94%D0%B5%D1%82%D1%81%D0%BA%D0%B0%D1%8F%20%D0%BF%D0%BB%D0%BE%D1%89%D0%B0%D0%B4%D0%BA%D0%B0&ll=43.575964%2C55.665986&mode=search&sctx=ZAAAAAgBEAAaKAoSCS7JAbuazkVAEa4tPC8V1UtAEhIJa378pUV9uj8RGEM50a5Coj8iBgABAgMEBSgKOABAy4kGSAFqAnJ1nQHNzMw9oAEAqAEAvQHhjsK2wgEGv8TGsKMCggIdKChjYXRlZ29yeV9pZDooODg4NDQ1NzU2OTMpKSmKAgs4ODg0NDU3NTY5M5ICAJoCDGRlc2t0b3AtbWFwcw%3D%3D&sll=43.575964%2C55.665986&sspn=0.011028%2C0.003801&text=%7B%22text%22%3A%22%D0%94%D0%B5%D1%82%D1%81%D0%BA%D0%B0%D1%8F%20%D0%BF%D0%BB%D0%BE%D1%89%D0%B0%D0%B4%D0%BA%D0%B0%22%2C%22what%22%3A%5B%7B%22attr_name%22%3A%22category_id%22%2C%22attr_values%22%3A%5B%2288844575693%22%5D%7D%5D%7D&z=16.84"),
                        },
                        []tgbotapi.InlineKeyboardButton{
                                 tgbotapi.NewInlineKeyboardButtonURL("🍂 Урочище Слуда (инклюзивная площадка)", "https://yandex.ru/maps/org/inklyuzivnaya_ploshchadka/95663808787/?ll=43.978557%2C56.280720&z=15.18"),
                        },
                )
                msg.ReplyMarkup = keyboard
                bot.Send(msg)
                return
        case strings.Contains(lowerText, "ноч"), strings.Contains(lowerText, "домик"), strings.Contains(lowerText, "отдых"), strings.Contains(lowerText, "отел"), strings.Contains(lowerText, "гостиниц"), strings.Contains(lowerText, "остановитьс"):
                msg := tgbotapi.NewMessage(message.Chat.ID, "Оборудованные домики и базы отдыха имеются здесь:")
                keyboard := tgbotapi.NewInlineKeyboardMarkup(
                        []tgbotapi.InlineKeyboardButton{
                                 tgbotapi.NewInlineKeyboardButtonURL("🏖 Ботанический заказник 'Пустынский'", "https://yandex.ru/maps/org/gorizont/178177856046/?display-text=%D0%91%D0%B0%D0%B7%D0%B0%2C%20%D0%B4%D0%BE%D0%BC%20%D0%BE%D1%82%D0%B4%D1%8B%D1%85%D0%B0&ll=43.526280%2C55.661593&mode=search&sll=43.586987%2C55.656154&sspn=0.114019%2C0.039308&text=%7B%22text%22%3A%22%D0%91%D0%B0%D0%B7%D0%B0%2C%20%D0%B4%D0%BE%D0%BC%20%D0%BE%D1%82%D0%B4%D1%8B%D1%85%D0%B0%22%2C%22what%22%3A%5B%7B%22attr_name%22%3A%22category_id%22%2C%22attr_values%22%3A%5B%22184106400%22%5D%7D%5D%7D&z=12.23"),
                        },
                        []tgbotapi.InlineKeyboardButton{
                                 tgbotapi.NewInlineKeyboardButtonURL("🌿 Керженский заповедник", "http://www.kerzhenskiy.ru/press-tsentr/novosti/?ELEMENT_ID=4998"),
                        },
                        []tgbotapi.InlineKeyboardButton{
                                 tgbotapi.NewInlineKeyboardButtonURL("🌳 Природный парк 'Воскресенское Поветлужье'", "https://yandex.ru/maps/org/vetluga/33674102220/?display-text=%D0%93%D0%BE%D1%81%D1%82%D0%B8%D0%BD%D0%B8%D1%86%D0%B0&ll=45.322477%2C56.944494&mode=search&sctx=ZAAAAAgBEAAaKAoSCQu45%2FnTwEZAEYNMMnIWbkxAEhIJ2UP7WMHv4z8RPsqIC0Cjyj8iBgABAgMEBSgKOABA2YkGSAFqAnJ1nQHNzMw9oAEAqAEAvQGjxefuwgEFzIuIuX2CAhsoKGNhdGVnb3J5X2lkOigxODQxMDY0MTQpKSmKAgkxODQxMDY0MTSSAgCaAgxkZXNrdG9wLW1hcHM%3D&sll=45.322477%2C56.944494&sspn=0.033200%2C0.011065&text=%7B%22text%22%3A%22%D0%93%D0%BE%D1%81%D1%82%D0%B8%D0%BD%D0%B8%D1%86%D0%B0%22%2C%22what%22%3A%5B%7B%22attr_name%22%3A%22category_id%22%2C%22attr_values%22%3A%5B%22184106414%22%5D%7D%5D%7D&z=15.25"),
                        },
                        []tgbotapi.InlineKeyboardButton{
                                 tgbotapi.NewInlineKeyboardButtonURL("🏚 Музей - заповедник 'Щелоковский хутор'", "https://yandex.ru/maps/org/les_park/223382717550/?display-text=%D0%93%D0%BE%D1%81%D1%82%D0%B8%D0%BD%D0%B8%D1%86%D0%B0&filter=alternate_vertical%3ARequestWindow&ll=43.997251%2C56.269976&mode=search&sctx=ZAAAAAgBEAAaKAoSCchAnl2%2BAUZAEWechqjCI0xAEhIJbXNjesISxT8RbxEY6xuYrD8iBgABAgMEBSgKOABAwpwGSAFqAnJ1nQHNzMw9oAEAqAEAvQGjxefuwgEt7oiXlcAGnbHyvpsB3KfZl4sG1%2BfChoEHzYLArosH%2Bbzcz5rX8Mo1iKHqi7MDggIbKChjYXRlZ29yeV9pZDooMTg0MTA2NDE0KSkpigIJMTg0MTA2NDE0kgIAmgIMZGVza3RvcC1tYXBzqgIiNzY1NzU1NzIsMjIwNDY4NTcyNjE4LDE3NTgyNzkyOTQ3NQ%3D%3D&sll=44.007401%2C56.268839&sspn=0.034134%2C0.011582&text=%7B%22text%22%3A%22%D0%93%D0%BE%D1%81%D1%82%D0%B8%D0%BD%D0%B8%D1%86%D0%B0%22%2C%22what%22%3A%5B%7B%22attr_name%22%3A%22category_id%22%2C%22attr_values%22%3A%5B%22184106414%22%5D%7D%5D%7D&z=15.21"),
                        },
                        []tgbotapi.InlineKeyboardButton{
                                 tgbotapi.NewInlineKeyboardButtonURL("🌲 Стригинский бор", "https://yandex.ru/maps/org/strigino_loft/200320243581/?display-text=%D0%91%D0%B0%D0%B7%D0%B0%2C%20%D0%B4%D0%BE%D0%BC%20%D0%BE%D1%82%D0%B4%D1%8B%D1%85%D0%B0&ll=43.789471%2C56.195897&mode=search&sll=43.789471%2C56.195897&sspn=0.054309%2C0.018463&text=%7B%22text%22%3A%22%D0%91%D0%B0%D0%B7%D0%B0%2C%20%D0%B4%D0%BE%D0%BC%20%D0%BE%D1%82%D0%B4%D1%8B%D1%85%D0%B0%22%2C%22what%22%3A%5B%7B%22attr_name%22%3A%22category_id%22%2C%22attr_values%22%3A%5B%22184106400%22%5D%7D%5D%7D&z=14.54"),
                        },
                )
                msg.ReplyMarkup = keyboard
                bot.Send(msg)
                return
        case strings.Contains(lowerText, "кон"), strings.Contains(lowerText, "клуб"):
                msg := tgbotapi.NewMessage(message.Chat.ID, "Покататься на лошадях вы сможете в:")
                keyboard := tgbotapi.NewInlineKeyboardMarkup(
                        []tgbotapi.InlineKeyboardButton{
                                 tgbotapi.NewInlineKeyboardButtonURL("🌳 Природный парк 'Воскресенское Поветлужье'", "https://yandex.ru/maps/?display-text=%D0%93%D0%BE%D1%81%D1%82%D0%B8%D0%BD%D0%B8%D1%86%D0%B0&ll=45.329494%2C56.944820&mode=search&poi%5Bpoint%5D=45.314261%2C56.944026&poi%5Buri%5D=ymapsbm1%3A%2F%2Forg%3Foid%3D181857306968&sctx=ZAAAAAgBEAAaKAoSCQu45%2FnTwEZAEYNMMnIWbkxAEhIJ2UP7WMHv4z8RPsqIC0Cjyj8iBgABAgMEBSgKOABA2YkGSAFqAnJ1nQHNzMw9oAEAqAEAvQGjxefuwgEFzIuIuX2CAhsoKGNhdGVnb3J5X2lkOigxODQxMDY0MTQpKSmKAgkxODQxMDY0MTSSAgCaAgxkZXNrdG9wLW1hcHM%3D&sll=45.329494%2C56.944820&sspn=0.059844%2C0.019944&text=%7B%22text%22%3A%22%D0%93%D0%BE%D1%81%D1%82%D0%B8%D0%BD%D0%B8%D1%86%D0%B0%22%2C%22what%22%3A%5B%7B%22attr_name%22%3A%22category_id%22%2C%22attr_values%22%3A%5B%22184106414%22%5D%7D%5D%7D&z=14.4"),
                        },
                )
                msg.ReplyMarkup = keyboard
                bot.Send(msg)
                return
        case strings.Contains(lowerText, "лыж"):
                msg := tgbotapi.NewMessage(message.Chat.ID, "⛷На лыжах можно покататься в следующих местах:")
                keyboard := tgbotapi.NewInlineKeyboardMarkup(
                        []tgbotapi.InlineKeyboardButton{
                                 tgbotapi.NewInlineKeyboardButtonURL("🏚 Музей - заповедник 'Щелоковский хутор'", "https://yandex.ru/maps/org/snezhinka/65639240880/?display-text=%D0%9B%D1%8B%D0%B6%D0%BD%D0%B0%D1%8F%20%D0%B1%D0%B0%D0%B7%D0%B0&ll=44.005202%2C56.271002&mode=search&sctx=ZAAAAAgBEAAaKAoSCUm8PJ0r5kVAEVK5iVqaGUxAEhIJPrSPFfw2lD8RZhNgWP58ez8iBgABAgMEBSgKOABAwpwGSAFqAnJ1nQHNzMw9oAEAqAEAvQEiB2JzwgESsKmdw%2FQB4s6N%2B6wGnLKc0%2FwFggIbKChjYXRlZ29yeV9pZDooMTg0MTA3MjkzKSkpigIJMTg0MTA3MjkzkgIAmgIMZGVza3RvcC1tYXBz&sll=44.012390%2C56.271002&sspn=0.020025%2C0.011662&text=%7B%22text%22%3A%22%D0%9B%D1%8B%D0%B6%D0%BD%D0%B0%D1%8F%20%D0%B1%D0%B0%D0%B7%D0%B0%22%2C%22what%22%3A%5B%7B%22attr_name%22%3A%22category_id%22%2C%22attr_values%22%3A%5B%22184107293%22%5D%7D%5D%7D&z=15.2"),
                        },
                       []tgbotapi.InlineKeyboardButton{
                                 tgbotapi.NewInlineKeyboardButtonURL("🌲 Стригинский бор", "https://yandex.ru/maps/org/lyzhnaya_baza_strigino/1593684396/?ll=43.798206%2C56.200023&z=16"),
                        }, 
                       []tgbotapi.InlineKeyboardButton{
                                 tgbotapi.NewInlineKeyboardButtonURL("🌿 Керженский заповедник", "http://www.kerzhenskiy.ru/press-tsentr/novosti/?ELEMENT_ID=5497"),
                        },
                )
                msg.ReplyMarkup = keyboard
                bot.Send(msg)
                return
        case strings.Contains(lowerText, "озер"), strings.Contains(lowerText, "пруд"), strings.Contains(lowerText, "пляж"), strings.Contains(lowerText, "купат"), strings.Contains(lowerText, "купан"):
                msg := tgbotapi.NewMessage(message.Chat.ID, "Купание разрешено в следующих местах:")
                keyboard := tgbotapi.NewInlineKeyboardMarkup(
                        []tgbotapi.InlineKeyboardButton{
                                 tgbotapi.NewInlineKeyboardButtonData("🌳 Природный парк 'Воскресенское Поветлужье'", "voskrMenu"),
                        },
                        []tgbotapi.InlineKeyboardButton{
                                 tgbotapi.NewInlineKeyboardButtonURL("🏚 Музей - заповедник 'Щелоковский хутор'", "https://yandex.ru/maps/47/nizhny-novgorod/?ll=44.016467%2C56.272634&mode=poi&poi%5Bpoint%5D=44.018422%2C56.274893&poi%5Buri%5D=ymapsbm1%3A%2F%2Forg%3Foid%3D20849343097&z=15.88"),
                        },
                        []tgbotapi.InlineKeyboardButton{
                                 tgbotapi.NewInlineKeyboardButtonURL("🏖 Ботанический заказник 'Пустынский'", "https://yandex.ru/maps/?ll=43.582114%2C55.664617&mode=poi&poi%5Bpoint%5D=43.583312%2C55.663998&poi%5Buri%5D=ymapsbm1%3A%2F%2Forg%3Foid%3D100000929578&z=17.11"),
                        },
                        []tgbotapi.InlineKeyboardButton{
                                 tgbotapi.NewInlineKeyboardButtonURL("🌲 Стригинский бор", "https://yandex.ru/maps/org/plyazh/57849500127/?display-text=%D0%9F%D0%BB%D1%8F%D0%B6&ll=43.782994%2C56.188543&mode=search&sctx=ZAAAAAgBEAAaKAoSCZbMsbyr5EVAESKl2TwOGUxAEhIJAFXcuMX8rD8R%2FDcvTny1kz8iBgABAgMEBSgKOABAvZwGSAFqAnJ1nQHNzMw9oAEAqAEAvQHf%2BjlWwgEp36PlwNcB78Kl4LICjLWruKkGy%2Fq%2B1NkFx4GkuVHyrZXP3wGo0rvUtwaCAhsoKGNhdGVnb3J5X2lkOigxODQxMDYzNDIpKSmKAgkxODQxMDYzNDKSAgCaAgxkZXNrdG9wLW1hcHM%3D&sll=43.782994%2C56.188543&sspn=0.048946%2C0.016643&text=%7B%22text%22%3A%22%D0%9F%D0%BB%D1%8F%D0%B6%22%2C%22what%22%3A%5B%7B%22attr_name%22%3A%22category_id%22%2C%22attr_values%22%3A%5B%22184106342%22%5D%7D%5D%7D&z=14.69"),
                        }, 
                )
                msg.ReplyMarkup = keyboard
                bot.Send(msg)
                return
        case strings.Contains(lowerText, "флор"), strings.Contains(lowerText, "фаун"), strings.Contains(lowerText, "животн"), strings.Contains(lowerText, "растен"):
                msg := tgbotapi.NewMessage(message.Chat.ID, "Разделы с флорой и фауной представлены в меню каждого объекта. Выберите интересующее место из главного меню.")
                bot.Send(msg)
                return
        case strings.Contains(lowerText, "поест"), strings.Contains(lowerText, "кафе"), strings.Contains(lowerText, "ресторан"), strings.Contains(lowerText, "столов"), strings.Contains(lowerText, "кушат"), strings.Contains(lowerText, "перекус"):
                msg := tgbotapi.NewMessage(message.Chat.ID, "Список кафе/ресторанов:")
                keyboard := tgbotapi.NewInlineKeyboardMarkup(
                        []tgbotapi.InlineKeyboardButton{
                                 tgbotapi.NewInlineKeyboardButtonURL("🍂 Урочище Слуда", "https://yandex.ru/maps/47/nizhny-novgorod/category/cafe/184106390/?ll=43.978628%2C56.282041&sctx=ZAAAAAgBEAAaKAoSCVXBqKRO5EVAETQw8rImGExAEhIJ8GlOXmQCrj8Rf03WqIdolD8iBgABAgMEBSgKOABAwpwGSAFqAnJ1nQHNzMw9oAEAqAEAvQFiVHEZwgE1zs7a3oIGo7m5kr8E2Jaqy2fUoan50wb4spCA%2BgaM7%2FKo6wb5o6Pb3gajkMztrQK31vyKrwKCAhsoKGNhdGVnb3J5X2lkOigxODQxMDYzOTApKSmKAgkxODQxMDYzOTCSAgCaAgxkZXNrdG9wLW1hcHOqAl8zNjcyMTY2MzY3LDEyODQ3NTU1MzcsMzY4NjYzNDE3NiwyMzcxMjU1NjcyMjMsMTE1MTQ5MDIyMTI3LDc4MDUzOTUzOTY1LDE3ODkwNTExNTIxMCw5MTg0OTU0OTI3Mw%3D%3D&sll=43.978628%2C56.282041&sspn=0.004326%2C0.001467&z=18.19"),
                        },
                        []tgbotapi.InlineKeyboardButton{
                                 tgbotapi.NewInlineKeyboardButtonURL("🌳 Природный парк 'Воскресенское Поветлужье'", "https://yandex.ru/maps/?display-text=%D0%9A%D0%B0%D1%84%D0%B5&ll=45.464123%2C56.851994&mode=search&sctx=ZAAAAAgBEAAaKAoSCRY0LbEyskZAEeUNMPMddkxAEhIJR%2Bhn6nWL6D8Ri1JCsKpe0D8iBgABAgMEBSgKOABA2YkGSAFqAnJ1nQHNzMw9oAEAqAEAvQFiVHEZwgEew%2BOqheAGgvyR6N0Ez8DGu4AB9uSEmv0B%2Fs3YlaMBggIbKChjYXRlZ29yeV9pZDooMTg0MTA2MzkwKSkpigIJMTg0MTA2MzkwkgIAmgIMZGVza3RvcC1tYXBz&sll=45.464123%2C56.851994&sspn=0.145324%2C0.048553&text=%7B%22text%22%3A%22%D0%9A%D0%B0%D1%84%D0%B5%22%2C%22what%22%3A%5B%7B%22attr_name%22%3A%22category_id%22%2C%22attr_values%22%3A%5B%22184106390%22%5D%7D%5D%7D&z=13.12"),
                        },
                        []tgbotapi.InlineKeyboardButton{
                                 tgbotapi.NewInlineKeyboardButtonData("🚲 Заповедник 'Сергачский дендропарк - Явлейка'", "sergach_cafe"),
                        },
                )
                msg.ReplyMarkup = keyboard
                bot.Send(msg)
                return
        default:
	        msg := tgbotapi.NewMessage(message.Chat.ID, "Не понял команду: " + message.Text)
	        bot.Send(msg)
        }
    }
}

func handleCallback(bot *tgbotapi.BotAPI, query *tgbotapi.CallbackQuery) {
	switch query.Data {
        case "striginoMenu":
                sendInlineMenu(bot, query.Message.Chat.ID, striginoMenu())
        case "selokovMenu":        
                sendInlineMenu(bot, query.Message.Chat.ID, selokovMenu())
        case "kerzh_info":
                sendPlaceDescription(bot, query.Message.Chat.ID, 1)
   
        case "voskr_info":
                sendPlaceDescription(bot, query.Message.Chat.ID, 2)

        case "selokov_info":
                sendPlaceDescription(bot, query.Message.Chat.ID, 3)

        case "ichalki_info":
                sendPlaceDescription(bot, query.Message.Chat.ID, 4)

        case "ichinfo":
                infoText := "<b>Что нужно взять с собой?</b>\n\n" + "<b>Еду</b> (ресторанов и кафе там нет).\n\n" +  "<b>Фонарики</b> (в пещерах темно).\n\n" + "<b>Средства от комаров и клещей</b> (вы приехали в лес).\n\n" + "<b>Удобную одежду и обувь</b> с нескользящей подошвой по сезону (в пещерах прохладно даже в 30-градусную жару).\n\n" + "❗❗❗Ехать в Ичалковский бор лучше в сухую ясную погоду. Поверхность склонов глинистая, поэтому в дождь сложно спуститься в пещеры, увеличивается риск травмироваться.\n\n"
                msg := tgbotapi.NewMessage(query.Message.Chat.ID, infoText)
                msg.ParseMode = "HTML"
                bot.Send(msg)

        case "pustyn_info":
                sendPlaceDescription(bot, query.Message.Chat.ID, 6)

        case "sergach_info":
                sendPlaceDescription(bot, query.Message.Chat.ID, 7)

        case "sergach_cafe":
                infoText := `<b>Кафе рядом с дендропарком:</b>

<b>1. Кафе «Астория»</b>
<a href="https://yandex.ru/maps/org/astoriya/32210541052/?ll=45.458671%2C55.520049&z=16">Открыть на карте</a>

<b>2. Кафе «Венеция»</b>
<a href="https://yandex.ru/maps/org/venetsiya/152057109048/?ll=45.446570%2C55.536079&z=16">Открыть на карте</a>

<b>3. Кафе «Чехов»</b>
<a href="https://yandex.ru/maps/org/kafe_chekhov/117583185792/?ll=45.456164%2C55.523795&z=16">Открыть на карте</a>`
                msg := tgbotapi.NewMessage(query.Message.Chat.ID, infoText)
                msg.ParseMode = "HTML"
                bot.Send(msg)

        case "sergachflora_info":
                sendPlaceSectionSplit(bot, query.Message.Chat.ID, 7, "flora")

        case "mux_info":
                sendPlaceDescription(bot, query.Message.Chat.ID, 8)

        case "mux_zapret":
                infoText := `<b>🚫Что запрещено/разрешено делать на территории?</b>

На всей территории заказника <b>запрещены</b> рубки леса (за исключением санитарных), подсочка деревьев, применение ядохимикатов ☢, распашка, строительство 🏗, добыча любых полезных ископаемых, геологоразведка, мелиоративные работы, водозабор и водоcброс. В особо защитных участках запрещается дополнительно прокладывание любых новых коммуникаций, отвод земель под любые виды пользования, сбор лекарственных и декоративных растений, разведение костров, весенняя охота, выпас скота. 

<b>Разрешается</b> охота в осенне-зимний период, лов рыбы удочкой и спиннингом 🎣, научные исследования, сенокошение, сбор грибов 🍄 и ягод 🍓.`
                msg := tgbotapi.NewMessage(query.Message.Chat.ID, infoText)
                msg.ParseMode = "HTML"
                bot.Send(msg)
   
        case "sluda_info":
                sendPlaceDescription(bot, query.Message.Chat.ID, 9)

        case "strigino_info":
                sendPlaceDescription(bot, query.Message.Chat.ID, 10)

        case "strigino_active":
                infoText := "Стригинский бор предлагает разнообразные возможности для активного отдыха. В зимнее время здесь функционирует лыжная база 🎿, где проводятся соревнования и марафоны 🏅. Летом тропы превращаются в маршруты для бегунов и велосипедистов 🚴‍♀️. Для любителей квадроциклов здесь круглый год работает база."
                msg := tgbotapi.NewMessage(query.Message.Chat.ID, infoText)
                bot.Send(msg)

        case "strigino_polza":
                infoText := `Прогулки по сосновым аллеям бора — это истинное наслаждение для души и тела. 

Чистый воздух 💨, насыщенный кислородом и фитонцидами, способствует укреплению иммунитета, а активные занятия ⛹️‍♂️ на природе улучшают кровообращение и общее самочувствие. 

В атмосфере спокойствия и умиротворения вы сможете снизить уровень стресса 🧘‍♂️ и зарядиться положительной энергией, которой так не хватает в городских джунглях.`
                msg := tgbotapi.NewMessage(query.Message.Chat.ID, infoText)
                msg.ParseMode = "HTML"
                bot.Send(msg)     

	case "kerzh_foto":
		sendPhotosByPlaceID(bot, query.Message.Chat.ID, 1)
     
        case "kerzhfauna_foto":
                sendPlaceSectionSplit(bot, query.Message.Chat.ID, 1, "fauna")


        case "voskr_foto":
                sendPhotosByPlaceID(bot, query.Message.Chat.ID, 2)

        case "sel_foto":
                imagePaths := []string{"sel1.jpg", "sel2.jpg", "sel3.jpg"}
                var mediaGroup []interface{}
                infoText := `<a href="https://hutormuzey.ru/index">Подробнее об объектах музея</a>`
                msg := tgbotapi.NewMessage(query.Message.Chat.ID, infoText)
                msg.ParseMode = "HTML"
                bot.Send(msg)
                for _, path := range imagePaths {
                        if _, err := os.Stat(path); err == nil {
                                media := tgbotapi.NewInputMediaPhoto(tgbotapi.FilePath(path))
                                mediaGroup = append(mediaGroup, media)
                        }
                }
                if len(mediaGroup) > 0 {
                        msg := tgbotapi.MediaGroupConfig {
                                ChatID: query.Message.Chat.ID,
                                Media: mediaGroup,
                        }
                        bot.SendMediaGroup(msg)
                }


        case "selokov_foto":
                sendPhotosByPlaceID(bot, query.Message.Chat.ID, 3)       

        case "ichalki_foto":
                sendPhotosByPlaceID(bot, query.Message.Chat.ID, 4)

        case "ichflora_foto":
                sendPlaceSectionSplit(bot, query.Message.Chat.ID, 4, "flora")

        case "ichfauna_foto":
                sendPlaceSectionSplit(bot, query.Message.Chat.ID, 4, "fauna")

        case "ichpe_info":
               infoText := "1️⃣Холодная или Ледяная пещера состоит из двух залов — Темного и Светлого. В подземном озере этой пещеры даже летом на дне сохраняется 15-сантиметровый слой льда.\n\n" + "2️⃣Безымянная или Малая ледяная пещера. Пожалуй, одна из самых красивых Ичалковских пещер. Ее скальные стены больше похожи не на природный объект, а на заброшенный старинный замок.\n\n" + "3️⃣Наклонная или Студенческая или Бутылочное горлышко находится в 110 м от Безымянной. Низкий узкий вход в пещеру можно найти в середине стены большого карстового лога. Проход постепенно расширяется и приводит к залу, где можно выпрямиться в полный рост.\n\n" + "4️⃣Теплая пещера. На дне пещеры температура не опускается ниже 3°С даже зимой, в большом зале есть подземное озеро глубиной 1,5 м. Считается, что в его воде содержится серебро, способствующее быстрому заживлению ран. Существует поверье, что умывшись из озера в Теплой пещере можно загадать желание, но всего лишь одно.\n\n" + "5️⃣Кулева яма (Кулемина или Кулевая яма). Находится в 400 м от Теплой пещеры, самый большой в Ичалковском бору карстовый провал, размерами 200 м на 150 м и глубиной до 50 м. С ним связано предание, что сюда сбрасывали завернутых в кули самоубийц.\n\n" + "6️⃣Старцева яма. Глубокий карстовый провал с отвесными стенками, в которых находятся три грота. Спуститься вниз можно только при помощи веревки или альпинистского оборудования. По легенде в одном из гротов жили старцы-отшельники, воду и пищу которым спускали на веревке.\n\n" + "7️⃣Лебяжьи переходы и Чертов мост — узкие перемычки между карстовыми провалами, сохранившиеся после обрушений известняковой породы. Чертов мост представляет собой живописную скальную арку и ведет к Безымянной пещере."
                msg := tgbotapi.NewMessage(query.Message.Chat.ID, infoText)
                bot.Send(msg)

        case "pustynflora_foto":
                sendPlaceSectionSplit(bot, query.Message.Chat.ID, 6, "flora")

        case "pustynfauna_foto":
                sendPlaceSectionSplit(bot, query.Message.Chat.ID, 6, "fauna")


        case "pustyn_foto":
                sendPhotosByPlaceID(bot, query.Message.Chat.ID, 6)

        case "sergach_foto":
                sendPhotosByPlaceID(bot, query.Message.Chat.ID, 7)
 
        case "muxflora_foto":
                sendPlaceSectionSplit(bot, query.Message.Chat.ID, 8, "flora")

        case "muxfauna_foto":
                sendPlaceSectionSplit(bot, query.Message.Chat.ID, 8, "fauna")


        case "mux_foto":
                sendPhotosByPlaceID(bot, query.Message.Chat.ID, 8)

        case "sluda_foto":
                sendPhotosByPlaceID(bot, query.Message.Chat.ID, 9)

        case "sludaflora_foto":
                sendPlaceSectionSplit(bot, query.Message.Chat.ID, 9, "flora")

        case "strigino_foto":
                sendPhotosByPlaceID(bot, query.Message.Chat.ID, 10)

        case "striginoflora_foto":
                sendPlaceSectionSplit(bot, query.Message.Chat.ID, 10, "flora")

        case "striginofauna_foto":
                sendPlaceSectionSplit(bot, query.Message.Chat.ID, 10, "fauna")

	}
}

func mainMenu() tgbotapi.ReplyKeyboardMarkup {
	return tgbotapi.NewReplyKeyboard(
		[]tgbotapi.KeyboardButton{{Text: "🌿 Керженский заповедник"}},
		[]tgbotapi.KeyboardButton{{Text: "🌳 Природный парк 'Воскресенское Поветлужье'"}},
		[]tgbotapi.KeyboardButton{{Text: "🏚 Музей - заповедник 'Щелоковский хутор'"}},
                []tgbotapi.KeyboardButton{{Text: "🏞 Ичалковский бор-заказник"}},
                []tgbotapi.KeyboardButton{{Text: "🏖 Ботанический заказник 'Пустынский'"}},
                []tgbotapi.KeyboardButton{{Text: "🚲 Заповедник 'Сергачский дендропарк - Явлейка'"}},
                []tgbotapi.KeyboardButton{{Text: "🌾 Мухтоловский заказник"}},
                []tgbotapi.KeyboardButton{{Text: "🍂 Урочище Слуда"}},
                []tgbotapi.KeyboardButton{{Text: "🌲 Стригинский бор"}},
                []tgbotapi.KeyboardButton{{Text: "🔍 Поиск по ключевым словам"}},
	)
}

func kerzhMenu() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		[]tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonURL("🗺 Место расположения", "https://yandex.ru/maps/org/ekotsentr_zapovednika_kerzhenskiy/203251058859/?ll=45.302881%2C56.501575&mode=search&sctx=ZAAAAAgBEAAaKAoSCVa7JqQ16EVAEbGH9rGCsUtAEhIJaqFkcmpnxD8Rf95UpMLYsj8iBgABAgMEBSgKOABA2YkGSAFqAnJ1nQHNzMw9oAEAqAEAvQGRCZewwgEMlN%2BN3a4Fq5HUlfUFggIp0LfQsNC%2F0L7QstC10LTQvdC40Log0LrQtdGA0LbQtdC90YHQutC40LmKAgCSAgCaAgxkZXNrdG9wLW1hcHM%3D&sll=45.302881%2C56.501575&source=serp_navig&sspn=1.732337%2C0.562214&text=%D0%B7%D0%B0%D0%BF%D0%BE%D0%B2%D0%B5%D0%B4%D0%BD%D0%B8%D0%BA%20%D0%BA%D0%B5%D1%80%D0%B6%D0%B5%D0%BD%D1%81%D0%BA%D0%B8%D0%B9&z=9"),
		},
		[]tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("❗ Информация о заповеднике", "kerzh_info"),
		},
                []tgbotapi.InlineKeyboardButton{
                        tgbotapi.NewInlineKeyboardButtonURL("🍃 Флора", "http://www.kerzhenskiy.ru/osnovnye-napravleniya-deyatelnosti/o-zapovednike/territoriya/rastitelnyy-mir/"),
                },
                []tgbotapi.InlineKeyboardButton{
                        tgbotapi.NewInlineKeyboardButtonData("🦉 Фауна", "kerzhfauna_foto"),
                 },
                []tgbotapi.InlineKeyboardButton{
                        tgbotapi.NewInlineKeyboardButtonURL("🖋 Запись на экскурсию", "http://www.kerzhenskiy.ru/osnovnye-napravleniya-deyatelnosti/ekoturizm/zayavka-na-ekskursiyu/"),
                },
                []tgbotapi.InlineKeyboardButton{
                        tgbotapi.NewInlineKeyboardButtonData("📸 Фотографии", "kerzh_foto"),
                },
		[]tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("🔙 Назад", "back"),
		},
	)
}

func voskrMenu() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
		[]tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonURL("🗺 Место расположения", "https://yandex.ru/maps/org/voskresenskoye_povetluzhye/1276282417/?ll=45.472169%2C56.952225&mode=search&sctx=ZAAAAAgBEAAaKAoSCVa7JqQ16EVAEbGH9rGCsUtAEhIJaqFkcmpnxD8Rf95UpMLYsj8iBgABAgMEBSgKOABA2YkGSAFqAnJ1nQHNzMw9oAEAqAEAvQFUtW5YwgEFsYzK4ASCAkvQstC%2B0YHQutGA0LXRgdC10L3RgdC60L7QtSDQv9C%2B0LLQtdGC0LvRg9C20YzQtSDQv9GA0LjRgNC%2B0LTQvdGL0Lkg0L%2FQsNGA0LqKAgCSAgCaAgxkZXNrdG9wLW1hcHM%3D&sll=45.472169%2C56.952225&source=serp_navig&sspn=0.616734%2C0.197760&text=%D0%B2%D0%BE%D1%81%D0%BA%D1%80%D0%B5%D1%81%D0%B5%D0%BD%D1%81%D0%BA%D0%BE%D0%B5%20%D0%BF%D0%BE%D0%B2%D0%B5%D1%82%D0%BB%D1%83%D0%B6%D1%8C%D0%B5%20%D0%BF%D1%80%D0%B8%D1%80%D0%BE%D0%B4%D0%BD%D1%8B%D0%B9%20%D0%BF%D0%B0%D1%80%D0%BA&z=11.09"),
		},
		[]tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("❗Информация о природном парке", "voskr_info"),
		},
                []tgbotapi.InlineKeyboardButton{
                        tgbotapi.NewInlineKeyboardButtonURL("📖 Сказки и легенды", "https://vizit-povetluzhie.ru/about/fairytails"),
                },
                []tgbotapi.InlineKeyboardButton{
                        tgbotapi.NewInlineKeyboardButtonURL("🚶Экотропы", "https://vizit-povetluzhie.ru/ecotrails"),
                },
                []tgbotapi.InlineKeyboardButton{
                        tgbotapi.NewInlineKeyboardButtonURL("🖋 Запись на экскурсию", "https://vizit-povetluzhie.ru/excursions"),
                },
                []tgbotapi.InlineKeyboardButton{
                        tgbotapi.NewInlineKeyboardButtonURL("🍯 Фермерская продукция", "https://vizit-povetluzhie.ru/farm-products"),
                },
                []tgbotapi.InlineKeyboardButton{
                        tgbotapi.NewInlineKeyboardButtonData("📸 Фотографии", "voskr_foto"),
                },
		[]tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("🔙 Назад", "back"),
		},
	)
}

func selokovMenu() tgbotapi.InlineKeyboardMarkup {
	return tgbotapi.NewInlineKeyboardMarkup(
                []tgbotapi.InlineKeyboardButton{
                        tgbotapi.NewInlineKeyboardButtonURL("🗺 Место расположения", "https://yandex.ru/maps/org/arkhitekturno_etnograficheskiy_muzey_zapovednik_shchyolokovskiy_khutor/1192859338/?ll=44.046581%2C56.275634&mode=search&sctx=ZAAAAAgAEAAaKAoSCVa7JqQ16EVAEbGH9rGCsUtAEhIJaqFkcmpnxD8Rf95UpMLYsj8iBgABAgMEBSgKOABAyFZIAWoCcnWdAc3MzD2gAQCoAQC9AeYggYbCAQEAggJD0YnQtdC70L7QutC%2B0LLRgdC60LjQuSDRhdGD0YLQvtGAINC90LjQttC90LXQs9C%2BINC90L7QstCz0L7RgNC%2B0LTQsIoCAJICBjEzMTM3N5oCDGRlc2t0b3AtbWFwcw%3D%3D&sll=44.013622%2C56.275634&source=serp_navig&sspn=0.164108%2C0.053578&text=%D1%89%D0%B5%D0%BB%D0%BE%D0%BA%D0%BE%D0%B2%D1%81%D0%BA%D0%B8%D0%B9%20%D1%85%D1%83%D1%82%D0%BE%D1%80%20%D0%BD%D0%B8%D0%B6%D0%BD%D0%B5%D0%B3%D0%BE%20%D0%BD%D0%BE%D0%B2%D0%B3%D0%BE%D1%80%D0%BE%D0%B4%D0%B0&z=13"),
                },
                []tgbotapi.InlineKeyboardButton{
                        tgbotapi.NewInlineKeyboardButtonData("❗Информация о музее - заповеднике", "selokov_info"),
                },
                []tgbotapi.InlineKeyboardButton{
                        tgbotapi.NewInlineKeyboardButtonURL("💰 Стоимость услуг", "https://vk.com/hutor_museum?w=wall-16938909_13506"),
                },
                []tgbotapi.InlineKeyboardButton{
                        tgbotapi.NewInlineKeyboardButtonData("🏚 Музей деревянного зодчества", "sel_foto"),
                },
		[]tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("📸 Фотографии", "selokov_foto"),
		},
               	[]tgbotapi.InlineKeyboardButton{
			tgbotapi.NewInlineKeyboardButtonData("🔙 Назад", "back"),
		},
	)
}

func ichalkiMenu() tgbotapi.InlineKeyboardMarkup {
        return tgbotapi.NewInlineKeyboardMarkup(
                []tgbotapi.InlineKeyboardButton{
                        tgbotapi.NewInlineKeyboardButtonURL("🗺 Место расположения", "https://yandex.ru/maps/org/ichalkovskiy_bor/202621226101/?ll=44.625439%2C55.447212&source=serp_navig&z=11.94"),
                },
                []tgbotapi.InlineKeyboardButton{
                        tgbotapi.NewInlineKeyboardButtonData("❗Информация о заказнике", "ichalki_info"),
                },
                []tgbotapi.InlineKeyboardButton{
                        tgbotapi.NewInlineKeyboardButtonData("🍃 Флора", "ichflora_foto"),
                },
                []tgbotapi.InlineKeyboardButton{
                        tgbotapi.NewInlineKeyboardButtonData("🦉 Фауна", "ichfauna_foto"),
                },
                []tgbotapi.InlineKeyboardButton{
                        tgbotapi.NewInlineKeyboardButtonData("⛰Пещеры", "ichpe_info"),
                },
                []tgbotapi.InlineKeyboardButton{
                        tgbotapi.NewInlineKeyboardButtonData("📸 Фотографии", "ichalki_foto"),
                },
                []tgbotapi.InlineKeyboardButton{
                        tgbotapi.NewInlineKeyboardButtonData("❗Что взять с собой?", "ichinfo"),
                },
                []tgbotapi.InlineKeyboardButton{
                        tgbotapi.NewInlineKeyboardButtonData("🔙 Назад", "back"),
                },
        )
}

func pustynMenu() tgbotapi.InlineKeyboardMarkup {
        return tgbotapi.NewInlineKeyboardMarkup(
                []tgbotapi.InlineKeyboardButton{
                        tgbotapi.NewInlineKeyboardButtonURL("🗺 Место расположения", "https://yandex.ru/maps/org/pustynskiy_gosudarstvenny_prirodny_zakaznik_regionalnogo_znacheniya/69544804621/?ll=43.608801%2C55.695398&mode=search&sll=43.608801%2C55.695377&source=serp_navig&text=%D0%B1%D0%BE%D1%82%D0%B0%D0%BD%D0%B8%D1%87%D0%B5%D1%81%D0%BA%D0%B8%D0%B9%20%D0%B7%D0%B0%D0%BA%D0%B0%D0%B7%D0%BD%D0%B8%D0%BA%20%D0%BF%D1%83%D1%81%D1%82%D0%BD%D1%8B%D0%BD%D1%81%D0%BA%D0%B8%D0%B9&z=12"),
                },
                []tgbotapi.InlineKeyboardButton{
                        tgbotapi.NewInlineKeyboardButtonData("❗Информация о заказнике", "pustyn_info"),
                },
                []tgbotapi.InlineKeyboardButton{
                        tgbotapi.NewInlineKeyboardButtonData("🍃 Флора", "pustynflora_foto"),
                },
                []tgbotapi.InlineKeyboardButton{
                        tgbotapi.NewInlineKeyboardButtonData("🦉 Фауна", "pustynfauna_foto"),
                },
                []tgbotapi.InlineKeyboardButton{
                        tgbotapi.NewInlineKeyboardButtonURL("🏚 База отдыха 'Горизонт'", "https://gorizontnn.tilda.ws/"),
                },
                []tgbotapi.InlineKeyboardButton{
                        tgbotapi.NewInlineKeyboardButtonData("📸 Фотографии", "pustyn_foto"),
                },
                []tgbotapi.InlineKeyboardButton{
                        tgbotapi.NewInlineKeyboardButtonData("🔙 Назад", "back"),
                },
        )
}

func sergachMenu() tgbotapi.InlineKeyboardMarkup {
        return tgbotapi.NewInlineKeyboardMarkup(
                []tgbotapi.InlineKeyboardButton{
                        tgbotapi.NewInlineKeyboardButtonURL("🗺 Место расположения", "https://yandex.ru/maps/11079/nizhny-novgorod-oblast'/geo/pamyatnik_prirody_regionalnogo_znacheniya_dendroparkovy_kompleks_sergachskogo_leskhoza_v_ovrage_yavleyka/3483763245/?ll=45.472948%2C55.544922&source=serp_navig&z=14.82"),
                },
                []tgbotapi.InlineKeyboardButton{
                        tgbotapi.NewInlineKeyboardButtonData("❗Информация о заповеднике", "sergach_info"),
                },
                []tgbotapi.InlineKeyboardButton{
                        tgbotapi.NewInlineKeyboardButtonData("🍃 Флора", "sergachflora_info"),
                },
                []tgbotapi.InlineKeyboardButton{
                        tgbotapi.NewInlineKeyboardButtonData("🍝 Кафе", "sergach_cafe"),
                },
                []tgbotapi.InlineKeyboardButton{
                        tgbotapi.NewInlineKeyboardButtonData("📸 Фотографии", "sergach_foto"),
                },
                []tgbotapi.InlineKeyboardButton{
                        tgbotapi.NewInlineKeyboardButtonData("🔙 Назад", "back"),
                },
        )
}

func muxMenu() tgbotapi.InlineKeyboardMarkup {
        return tgbotapi.NewInlineKeyboardMarkup(
                []tgbotapi.InlineKeyboardButton{
                        tgbotapi.NewInlineKeyboardButtonURL("🗺 Место расположения", "https://yandex.ru/maps/?ll=43.240970%2C55.502229&mode=whatshere&source=serp_navig&whatshere%5Bpoint%5D=43.198089%2C55.499828&whatshere%5Bzoom%5D=12.24&z=12.24"),
                },
                []tgbotapi.InlineKeyboardButton{
                        tgbotapi.NewInlineKeyboardButtonData("❗Информация о заказнике", "mux_info"),
                },
                []tgbotapi.InlineKeyboardButton{
                        tgbotapi.NewInlineKeyboardButtonData("🚫 Что запрещено/разрешено делать на территории?", "mux_zapret"),
                },
                []tgbotapi.InlineKeyboardButton{
                        tgbotapi.NewInlineKeyboardButtonData("🍃 Флора", "muxflora_foto"),
                },
                []tgbotapi.InlineKeyboardButton{
                        tgbotapi.NewInlineKeyboardButtonData("🦉 Фауна", "muxfauna_foto"),
                },
                []tgbotapi.InlineKeyboardButton{
                        tgbotapi.NewInlineKeyboardButtonData("📸 Фотографии", "mux_foto"),
                },
                []tgbotapi.InlineKeyboardButton{
                        tgbotapi.NewInlineKeyboardButtonData("🔙 Назад", "back"),
                },
        )
}

func sludaMenu() tgbotapi.InlineKeyboardMarkup {
        return tgbotapi.NewInlineKeyboardMarkup(
                []tgbotapi.InlineKeyboardButton{
                        tgbotapi.NewInlineKeyboardButtonURL("🗺 Место расположения", "https://yandex.ru/maps/org/urochishche_sluda/74699685317/?ll=43.973293%2C56.278028&source=serp_navig&z=13.36"),
                },
                []tgbotapi.InlineKeyboardButton{
                        tgbotapi.NewInlineKeyboardButtonData("❗Информация о заповеднике", "sluda_info"),
                },
                []tgbotapi.InlineKeyboardButton{
                        tgbotapi.NewInlineKeyboardButtonURL("📜 История", "https://swissparknn.ru/sluda-urochishhe-zapovednoe-mesto-chto-eto-za-nazvaniya-dikovinnye-i-prostranstvo-zagadochnoe/"),
                },
                []tgbotapi.InlineKeyboardButton{
                        tgbotapi.NewInlineKeyboardButtonData("🍃 Флора", "sludaflora_foto"),
                },
                []tgbotapi.InlineKeyboardButton{
                        tgbotapi.NewInlineKeyboardButtonData("📸 Фотографии", "sluda_foto"),
                },
                []tgbotapi.InlineKeyboardButton{
                        tgbotapi.NewInlineKeyboardButtonData("🔙 Назад", "back"),
                },
        )
}

func striginoMenu() tgbotapi.InlineKeyboardMarkup {
        return tgbotapi.NewInlineKeyboardMarkup(
                []tgbotapi.InlineKeyboardButton{
                        tgbotapi.NewInlineKeyboardButtonURL("🗺 Место расположения", "https://yandex.ru/maps/47/nizhny-novgorod/geo/pamyatnik_prirody_regionalnogo_oblastnogo_znacheniya_striginskiy_bor/120897927/?ll=43.785284%2C56.195567&source=serp_navig&z=15.19"),
                },
                []tgbotapi.InlineKeyboardButton{
                        tgbotapi.NewInlineKeyboardButtonData("❗Информация о памятнике природы", "strigino_info"),
                },
                []tgbotapi.InlineKeyboardButton{
                        tgbotapi.NewInlineKeyboardButtonData("🚴‍♂️ Активный отдых", "strigino_active"),
                },
                []tgbotapi.InlineKeyboardButton{
                        tgbotapi.NewInlineKeyboardButtonData("🍃 Флора", "striginoflora_foto"),
                },
                []tgbotapi.InlineKeyboardButton{
                        tgbotapi.NewInlineKeyboardButtonData("🦉 Фауна", "striginofauna_foto"),
                },
                []tgbotapi.InlineKeyboardButton{
                        tgbotapi.NewInlineKeyboardButtonData("👩‍⚕️ Польза для здоровья", "strigino_polza"),
                },
                []tgbotapi.InlineKeyboardButton{
                        tgbotapi.NewInlineKeyboardButtonData("📸 Фотографии", "strigino_foto"),
                },
                []tgbotapi.InlineKeyboardButton{
                        tgbotapi.NewInlineKeyboardButtonData("🔙 Назад", "back"),
                },
        )
}


func sendInlineMenu(bot *tgbotapi.BotAPI, chatID int64, menu tgbotapi.InlineKeyboardMarkup) {
	msg := tgbotapi.NewMessage(chatID, "Выберите категорию:")
	msg.ReplyMarkup = menu
	bot.Send(msg)
}
