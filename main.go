package main

import (
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"sky9464/ykt"
	"time"

	"github.com/gorilla/websocket"
	"github.com/tidwall/gjson"
	"gopkg.in/yaml.v2"
)

// readConfig 读取config.yml文件并解析为Config结构体
func ReadConfig(filePath string) (Config, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return Config{}, err
	}
	defer file.Close()

	var config Config
	decoder := yaml.NewDecoder(file)
	err = decoder.Decode(&config)
	if err != nil {
		return Config{}, err
	}

	return config, nil
}
func reconnectWebSocket(lessonToken, identityId, lessonId string) (*websocket.Conn, error) {
	wsURL := "wss://www.yuketang.cn/wsapp/"
	u, err := url.Parse(wsURL)
	if err != nil {
		return nil, fmt.Errorf("url parse: %v", err)
	}

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("websocket dial: %v", err)
	}

	// 重新发送初始消息
	message := fmt.Sprintf(`{"op":"hello","userid":%s,"role":"student","auth":"%s","lessonid":"%s"}`, identityId, lessonToken, lessonId)
	err = c.WriteMessage(websocket.TextMessage, []byte(message))
	if err != nil {
		c.Close()
		return nil, fmt.Errorf("writing message error: %v", err)
	}

	return c, nil
}

// 监听消息并处理
func ListenForMessages(c *websocket.Conn, auth string, sessionid string, exitChan chan struct{}, problemDelay map[string]int, identityId string, lessonToken string, lessonId string) {
	defer c.Close()

	for {
		_, message, err := c.ReadMessage() // 持续监听
		if err != nil {
			log.Println("消息读取错误:", err)
			c, err = reconnectWebSocket(lessonToken, identityId, lessonId)
			if err != nil {
				log.Printf("重新连接WebSocket失败: %v,即将退出程序\n", err)
				close(exitChan)
				return
			}
			continue
		}

		// 打印接收到的消息
		log.Printf("%s\n", message)

		// 使用 gjson 解析接收到的 JSON 字符串
		response := string(message)
		op := gjson.Get(response, "op").String()

		// 根据 op 的类型进行不同的处理
		switch op {
		case "hello":
			// time.Sleep(3 * time.Second) // 等待3秒
			// 1.hello握手（未放映结束）     ： {"op":"hello","lessonid":"1239590491528391040","livestatus":0,"livevoice":0,"timeline":[],"addinversion":5.2,"interactive":false,"danmu":false,"presentation":"1239590602534865280","slideindex":47,"slideid":"1239590602702637441","unlockedproblem":[]}
			// 如果presentation字段存在，则提取该字段
			presentation_id := gjson.Get(response, "presentation").String()
			if presentation_id != "" {
				log.Printf("------------当前正在播放编号为: %s的PPT-----------------\n", presentation_id)
				err := ykt.StorePPT(sessionid, presentation_id, auth)
				if err != nil {
					log.Printf("Error: %v\n", err)
				}
			} else {
				// 如果为空，则获取timeline中的最后一条中的pres字段
				timeLine := gjson.Get(response, "timeline").Array()
				if len(timeLine) > 3 {
					presentation_id := timeLine[len(timeLine)-2].Get("pres").String()
					if presentation_id != "" {
						log.Printf("-------------------------编号为%s的PPT已经放映完毕或暂未放映----------------\n", presentation_id)
						err := ykt.StorePPT(sessionid, presentation_id, auth)
						if err != nil {
							log.Printf("Error: %v\n", err)
						}
					}
				}
			}
			// 获取unlockedproblem中的问题，补偿提交
			unlockedproblem := gjson.Get(response, "unlockedproblem").Array()
			ykt.CompensateSubmit(sessionid, auth, unlockedproblem, problemDelay)
		case "fetchtimeline":
			// 消息内容示例 {"op":"fetchtimeline","msgid":1,"lessonid":"1239590491528391040","timeline":[],"presentation":"1239590602534865280","slideindex":47,"slideid":"1239590602702637441","danmu":false,"unlockedproblem":[]}
			presentation_id := gjson.Get(response, "presentation").String()
			if presentation_id != "" {
				log.Printf("当前正在播放: %s\n", presentation_id)
				err := ykt.StorePPT(sessionid, presentation_id, auth)
				if err != nil {
					log.Printf("Error: %v\n", err)
				}
				// 获取PPT
			}
			// 获取unlockedproblem中的问题，补偿提交
			unlockedproblem := gjson.Get(response, "unlockedproblem").Array()
			ykt.CompensateSubmit(sessionid, auth, unlockedproblem, problemDelay)
		case "showpresentation":
			// 消息内容示例：{"op":"showpresentation","lessonid":"1237970788183733504","presentation":"1238019568283191296","dt":1725420236240,"timeline":[{"type":"event","title":"新幻灯片: 第四章测试题","code":"SHOW_PRESENTATION","replace":["第四章测试题"],"dt":1725420236242},{"type":"slide","pres":"1238019568283191296","si":1,"sid":"1238019568291579904","step":-1,"total":0,"dt":1725420236242}],"unlockedproblem":[],"slideindex":1,"slideid":"1238019568291579904","shownow":true,"danmu":false}
			presentation_id := gjson.Get(response, "presentation").String()
			if presentation_id != "" {
				log.Printf("当前正在播放: %s\n", presentation_id)
				err := ykt.StorePPT(sessionid, presentation_id, auth)
				if err != nil {
					log.Printf("Error: %v\n", err)
				}
				// 获取PPT
			}
			// // 获取unlockedproblem中的问题，补偿提交
			// unlockedproblem := gjson.Get(response, "unlockedproblem").Array()
			// CompensateSubmit(sessionid, auth, unlockedproblem)
		case "presentationcreated":
			// 消息内容示例：{"op":"presentationcreated","lessonid":"1237970788183733504","presentation":"1237972820995355008"}
			presentation_id := gjson.Get(response, "presentation").String()
			if presentation_id != "" {
				log.Printf("------------老师上传了新的PPT，编号为%s--------------", presentation_id)
				err := ykt.StorePPT(sessionid, presentation_id, auth)
				if err != nil {
					log.Printf("Error: %v\n", err)
				}
				// 获取PPT
			}
			// {"op":"fetchtimeline","lessonid":"1253416309929082368","msgid":1}
			// 发送消息给websocket连接
			message := fmt.Sprintf(`{"op":"fetchtimeline","lessonid":"%s","msgid":1}`, lessonId)
			err = c.WriteMessage(websocket.TextMessage, []byte(message))
			if err != nil {
				log.Printf("Error: %v\n", err)
			}
		case "presentationupdated":
			// 消息内容示例：{"op":"presentationupdated","lessonid":"1240527675479630208","presentation":"1240529739052226560"}
			presentation_id := gjson.Get(response, "presentation").String()
			if presentation_id != "" {
				log.Printf("--------------编号为%s的PPT已更新--------------", presentation_id)
				err := ykt.StorePPT(sessionid, presentation_id, auth)
				if err != nil {
					log.Printf("Error: %v\n", err)
				}
				// 获取PPT
			}
			// {"op":"fetchtimeline","lessonid":"1253416309929082368","msgid":1}
			message := fmt.Sprintf(`{"op":"fetchtimeline","lessonid":"%s","msgid":1}`, lessonId)
			err = c.WriteMessage(websocket.TextMessage, []byte(message))
			if err != nil {
				log.Printf("Error: %v\n", err)
			}
		case "slidenav":
			// 消息内容示例：{"op":"slidenav","lessonid":"1237970788183733504","unlockedproblem":["1238019568291579905","1238019568291579906","1238019568299968512","1238019568299968513","1238019568299968514"],"danmu":false,"shownow":true,"to":"student","slide":{"type":"slide","pres":"1238019568283191296","si":7,"sid":"1238019568299968515","step":-1,"total":0,"dt":1725420529224},"im":false}
			// 提取pres字段作为presentation_id
			presentation_id := gjson.Get(response, "slide.pres").String()
			slidenav := gjson.Get(response, "slide.sid").String()
			log.Printf("--------------当前幻灯片ID为%s，PPT编号为%s--------------", slidenav, presentation_id)
			if presentation_id != "" {
				log.Printf("--------------当前正在播放编号为%s的PPT--------------", presentation_id)
				err := ykt.StorePPT(sessionid, presentation_id, auth)
				if err != nil {
					log.Printf("Error: %v\n", err)
				}
				// 获取PPT
			}
			// 获取unlockedproblem中的问题，补偿提交
			unlockedproblem := gjson.Get(response, "unlockedproblem").Array()
			ykt.CompensateSubmit(sessionid, auth, unlockedproblem, problemDelay)
		case "unlockproblem":
			// 消息内容示例 ： {"op":"unlockproblem","lessonid":"1237970788183733504","problem":{"type":"problem","prob":"1238019568299968515","pres":"1238019568283191296","si":6,"sid":"1238019568299968515","dt":1725420531095,"limit":45},"unlockedproblem":["1238019568291579905","1238019568291579906","1238019568299968512","1238019568299968513","1238019568299968514","1238019568299968515"]}
			problemId := gjson.Get(response, "problem.prob").String()
			// {"op":"probleminfo","lessonid":"1253416309929082368","problemid":"1253416863359064193","msgid":1}
			message := fmt.Sprintf(`{"op":"probleminfo","lessonid":"%s","problemid":"%s","msgid":1}`, lessonId, problemId)
			err = c.WriteMessage(websocket.TextMessage, []byte(message))
			if err != nil {
				log.Printf("Error: %v\n", err)
			}
			// 提取 pres 字段作为 presentation_id
			presentation_id := gjson.Get(response, "problem.pres").String()
			if presentation_id != "" {
				log.Printf("--------------当前正在播放编号为%s的PPT--------------", presentation_id)
				err := ykt.StorePPT(sessionid, presentation_id, auth)
				if err != nil {
					log.Printf("Error: %v\n", err)
				}
				// 获取PPT
			}
			ykt.PostAnswer(sessionid, auth, problemId, problemDelay, 1)
		case "probleminfo":
			// 消息内容示例: {"op":"probleminfo","msgid":5,"lessonid":"1237970788183733504","problemid":"1238019568299968514","dt":1725420483986,"limit":45,"now":1725420488857}
			// 提交当前问题的答案
			problemId := gjson.Get(response, "problemid").String()
			ykt.PostAnswer(sessionid, auth, problemId, problemDelay, 1)
		case "extendtime":
			// 消息内容示例： {"op":"extendtime","lessonid":"1240465636950545792","problem":{"type":"problem","prob":"1240489299854759936","pres":"1240466501304325504","si":null,"sid":"1240489299854759936","dt":1725714703512,"limit":1029,"extend":60,"now":1725715673344}}
			log.Printf("--------------老师把这道题延长了%d秒，这辈子有了--------------", gjson.Get(response, "problem.extend").Int())
			// 提取pres字段作为presentation_id
			presentation_id := gjson.Get(response, "problem.pres").String()
			if presentation_id != "" {
				log.Printf("--------------当前正在播放编号为%s的PPT--------------", presentation_id)
				err := ykt.StorePPT(sessionid, presentation_id, auth)
				if err != nil {
					log.Printf("Error: %v\n", err)
				}
				// 获取PPT
			}
			// 获取problem中的prob字段
			problemId := gjson.Get(response, "problem.prob").String()
			ykt.PostAnswer(sessionid, auth, problemId, problemDelay, 0)
		case "problemfinished":
			// 消息内容示例 ： {"op":"problemfinished","lessonid":"1237970788183733504","prob":"1238019568291579906","pres":"1238019568283191296","sid":"1238019568291579906","dt":1725420352409,"problem":{"type":"problem","prob":"1238019568291579906","pres":"1238019568283191296","si":null,"sid":"1238019568291579906","dt":1725420352409,"limit":0}}
			// 提取prob字段作为problemId
			problemId := gjson.Get(response, "prob").String()
			log.Printf("--------------编号为%s的问题已提交--------------", problemId)
			// 提取 pres 字段作为 presentation_id
			presentation_id := gjson.Get(response, "pres").String()
			if presentation_id != "" {
				log.Printf("--------------当前PPT编号为%s--------------", presentation_id)
				err := ykt.StorePPT(sessionid, presentation_id, auth)
				if err != nil {
					log.Printf("Error: %v\n", err)
				}
				// 获取PPT
			}
		case "callpaused":
			// 消息内容示例：{"op":"callpaused","msgid":1726020062518,"lessonid":"1243046922772946688","uid":57336256,"name":"宋癸军","avatar":"https://thirdwx.qlogo.cn/mmopen/vi_32/rV1l0Zt2zpN6RCF3Wap1U96KsFibIeGbFcsK1NPtt5qRmot5GyISyiadA46gpXOygxkNzwAwsha0TibjrjFlRh9lA/132","sid":"2022065229","dt":1726020089531,"event":{"type":"event","title":"随机点名选中：宋癸军 2022065229","code":"RANDOM_PICK","replace":["宋癸军","2022065229"],"dt":1726020089531}}
			// 输出title字段
			title := gjson.Get(response, "event.title").String()
			log.Printf("%s\n", title)
		case "showfinished":
			// 消息内容示例 : {"op":"showfinished","lessonid":"1239590491528391040","presentation":"1239590602534865280","event":{"type":"event","title":"幻灯片 电路基础 结束放映","code":"SHOW_FINISH","replace":["电路基础"],"dt":1725613247089}}
			log.Println("----------------------------------当前PPT播放结束-----------------------------")
			presentation_id := gjson.Get(response, "presentation").String()
			if presentation_id != "" {
				log.Printf("--------------当前正在播放编号为%s的PPT--------------", presentation_id)
				err := ykt.StorePPT(sessionid, presentation_id, auth)
				if err != nil {
					log.Printf("Error: %v\n", err)
				}
				// 获取PPT
			}
		case "lessonfinished":
			// 消息内容示例 ：{"op":"lessonfinished","lessonid":"1239590491528391040","event":{"type":"event","title":"下课啦！","code":"LESSON_FINISH","dt":1725613711299}}
			log.Println("课程已结束，退出程序")
			close(exitChan) // 发送退出信号
			return

		default:
			log.Printf("接收到未知消息: %s\n", response)
		}
	}
}

// Config 定义配置文件的结构
type Config struct {
	// sessionId: ""
	// courseId: "4201115"
	// starttime: 2004-12-26T05:13:14
	// endtime: 2024-11-02T00:00:00
	// ProblemDelay:
	// 	min: 10
	// 	max: 15
	SessionId       string `yaml:"sessionId"`
	CourseId        string `yaml:"courseId"`
	StartTime       string `yaml:"starttime"`
	EndTime         string `yaml:"endtime"`
	ProblemDelayMin int    `yaml:"ProblemDelayMin"`
	ProblemDelayMax int    `yaml:"ProblemDelayMax"`
}

func InitFiles() {
	// 检测这些文件是否存在
	if _, err := os.Stat("./ppts"); os.IsNotExist(err) {
		// 创建ppts文件夹
		os.MkdirAll("./ppts", os.ModePerm)
		log.Println("-------------创建ppts文件夹成功-------------")
	}
	if _, err := os.Stat("./bank.json"); os.IsNotExist(err) {
		// 创建bank.json文件
		os.Create("./bank.json")
		// 写入 [],并保存
		os.WriteFile("./bank.json", []byte("[]"), 0644)
		log.Println("-------------创建bank.json文件成功-------------")
	}
	if _, err := os.Stat("./config.yml"); os.IsNotExist(err) {
		// 创建config.yml文件,并写入默认配置
		file, err := os.Create("./config.yml")
		if err != nil {
			log.Fatalf("error: %v", err)
		}
		defer file.Close()
		encoder := yaml.NewEncoder(file)
		err = encoder.Encode(Config{
			SessionId:       "",
			CourseId:        "4201115",
			StartTime:       "2004-12-26T05:20",
			EndTime:         "2024-11-02T00:00",
			ProblemDelayMin: 10,
			ProblemDelayMax: 15,
		})
		// 在config.yml文件中写点儿注释
		file.WriteString("# 参数详解\n")
		file.WriteString("# SessionId: 你的sessionId             不用管它，扫码后会自动生成\n")
		file.WriteString("# CourseId: 你的课程Id                 扫码登录后会生成一个courseId.txt的文件，里面有该账号下所有的课程Id\n")
		file.WriteString("# StartTime: 你的课程开始时间          格式为2004-12-26T05:13:14\n")
		file.WriteString("# EndTime: 你的课程结束时间            格式为2004-12-26T05:13:14\n")
		file.WriteString("# ProblemDelayMin: 你的题目提交延时最小时间    单位为秒\n")
		file.WriteString("# ProblemDelayMax: 你的题目提交延时最大时间    单位为秒\n")
		if err != nil {
			log.Fatalf("error: %v", err)
		}
		log.Println("-------------创建config.yml文件成功-------------")
	}
}

func CheckTime(StartTime, EndTime string) (*time.Location, time.Time, time.Time) {
	// 加载UTC+8(使用CST的偏移量 +80.00)
	loc := time.FixedZone("CST", 8*3600)

	// 解析时间，使用北京时间
	startTime, err := time.ParseInLocation("2006-01-02T15:04", StartTime, loc)
	if err != nil {
		log.Fatalf("error parsing start time: %v", err)
	}

	endTime, err := time.ParseInLocation("2006-01-02T15:04", EndTime, loc)
	if err != nil {
		log.Fatalf("error parsing end time: %v", err)
	}

	currentTime := time.Now().In(loc) // 获取当前北京时间

	// 检查当前时间是否在开始和结束时间之间
	if currentTime.After(endTime) {
		log.Println("当前时间已超过结束时间，程序退出。")
		os.Exit(0)
	} else if currentTime.Before(startTime) {
		// 等待直到开始时间
		log.Printf("尚未到达开始时间，等待到 %s\n", startTime)
		time.Sleep(time.Until(startTime))
	}

	// 在时间段内继续执行下面的代码
	log.Println("---------------当前时间在配置好的时间段内，继续执行----------------")
	// 返回 loc 、startTime 、endTime
	return loc, startTime, endTime
}
func main() {
	InitFiles()
	// 创建多重输出
	logFile, err := os.OpenFile("log.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("error opening log file: %v", err)
	}
	defer logFile.Close()

	multiWriter := io.MultiWriter(os.Stdout, logFile)
	log.SetOutput(multiWriter)
	log.SetFlags(log.Ldate | log.Ltime)
	// log.Println("-------------日志输出位置设置成功-------------")

	// 读取配置文件config.yml 获取sessionId、courseId、starttime、endtime
	config, err := ReadConfig("config.yml")
	if err != nil {
		log.Fatalf("error: %v", err)
	}
	log.Println("============================================================================================================")
	log.Println("===================================配置文件config.yml内容如下===============================================")
	log.Printf("SessionId: %s\n", config.SessionId)
	log.Printf("CourseId: %s\n", config.CourseId)
	log.Printf("StartTime: %s\n", config.StartTime)
	log.Printf("EndTime: %s\n", config.EndTime)
	log.Printf("ProblemDelayMin: %d\n", config.ProblemDelayMin)
	log.Printf("ProblemDelayMax: %d\n", config.ProblemDelayMax)
	log.Println("============================================================================================================")

	// 检查当前sessionId是否过期
	if !ykt.CheckSessionId(config.SessionId) {
		log.Println("当前sessionId已过期，请重新登录获取新的sessionId。")
		sessionId, err := ykt.QrLogin()
		if err != nil {
			log.Fatalf("error: %v", err)
		}
		config.SessionId = sessionId
		// 保存sessionId到配置文件
		file, err := os.OpenFile("config.yml", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			log.Fatalf("error: %v", err)
		}
		defer file.Close()
		encoder := yaml.NewEncoder(file)
		err = encoder.Encode(config)
		if err != nil {
			log.Fatalf("error: %v", err)
		}
		log.Println("sessionId已更新，请重新运行程序。")
		os.Exit(0)
	}
	loc, _, endTime := CheckTime(config.StartTime, config.EndTime)
	log.Printf("根据配置信息，程序将在 %s 结束\n", endTime)
	ykt.SaveCourseId(config.SessionId)
	lessonId := ykt.GetLessonId(config.SessionId, config.CourseId)
	// log.Printf("LessonId: %s\n", lessonId)
	// 获取更多关于课程的信息
	lessonToken, identityId, auth := ykt.GetLessonOtherInfo(config.SessionId, lessonId)
	if err != nil {
		log.Fatalf("error: %v", err)
	}
	// log.Printf("LessonToken: %s\n", lessonToken)
	// log.Printf("IdentityId: %s\n", identityId)
	// log.Printf("Auth: %s\n", auth)

	// 开启监听websocket
	// 开启WebSocket连接
	wsURL := "wss://www.yuketang.cn/wsapp/"
	u, err := url.Parse(wsURL)
	if err != nil {
		log.Fatalf("url parse: %v", err)
	}

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		log.Fatalf("websocket dial: %v", err)
	}
	// 发送初始消息
	message := fmt.Sprintf(`{"op":"hello","userid":%s,"role":"student","auth":"%s","lessonid":"%s"}`, identityId, lessonToken, lessonId)
	err = c.WriteMessage(websocket.TextMessage, []byte(message))
	if err != nil {
		log.Fatalf("writing message error: %v", err)
	}
	// 创建一个频道用于控制退出
	exitChan := make(chan struct{})

	// 将config.ProblemDelay转换为int
	problemDelay := map[string]int{
		"min": config.ProblemDelayMin,
		"max": config.ProblemDelayMax,
	}
	// 转成map[string]int
	problemDelayInt := map[string]int{
		"min": problemDelay["min"],
		"max": problemDelay["max"],
	}
	// 作为一个范围参数传进去
	// 启动消息监听
	go ListenForMessages(c, auth, config.SessionId, exitChan, problemDelayInt, identityId, lessonToken, lessonId)

	// 持续检查结束条件
	for {
		select {
		case <-exitChan:
			log.Println("监听已被打断，退出程序。")
			return
		default:
			currentTime := time.Now().In(loc) // 获取当前北京时间
			// 将endTime写死成2024-09-18T00:00:00
			// endTime, err := time.ParseInLocation("2006-01-02T15:04", "2024-11-02T00:00", loc)
			if err != nil {
				log.Fatalf("error parsing end time: %v", err)
			}
			if currentTime.After(endTime) {
				log.Println("---------------程序结束，即将退出程序---------------")
				return
			}
			time.Sleep(3 * time.Second) // 为了避免CPU占用过高，适当地睡眠
			// log.Println("连接成功")

		}
	}

}
