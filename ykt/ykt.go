package ykt

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"math/rand"

	"github.com/gorilla/websocket"
	"github.com/skip2/go-qrcode"
	"github.com/tidwall/gjson"
)

// Qrlogin 函数尝试通过二维码登录，并返回 sessionid。
// 如果成功，它还会将用户信息保存到 SQLite 数据库中。
func QrLogin() (string, error) {
	wsURL := "wss://www.yuketang.cn/wsapp/"
	u, err := url.Parse(wsURL)
	if err != nil {
		return "", fmt.Errorf("url parse: %v", err)
	}

	c, resp, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return "", fmt.Errorf("websocket dial: %v", err)
	}
	defer c.Close()
	log.Printf("建立二维码登录websocket连接|响应状态码: %d", resp.StatusCode)

	jsonData := `{
		"op": "requestlogin",
		"role": "web",
		"version": 1.4,
		"type": "qrcode",
		"from": "web"
	}`

	err = c.WriteMessage(websocket.TextMessage, []byte(jsonData))
	if err != nil {
		return "", fmt.Errorf("write message: %v", err)
	}
	sessionIdChan := make(chan string)
	errChan := make(chan error)
	done := make(chan struct{})
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				log.Println("read:", err)
				done <- struct{}{}
				return
			}

			response := string(message)

			op := gjson.Get(response, "op").String()
			if op == "" {
				return
			}

			switch op {
			case "requestlogin":
				ticket := gjson.Get(response, "ticket").String()
				if ticket == "" {
					return
				}
				ShowQrcode(ticket) // 传递ticket而不是qrCode

			case "loginsuccess":
				UserID := gjson.Get(response, "UserID").Int()
				Name := gjson.Get(response, "Name").String()
				LastLogin := gjson.Get(response, "LastLogin").String()
				Department := gjson.Get(response, "Department").String()

				LastLoginIP := gjson.Get(response, "LastLoginIP").String()

				Auth := gjson.Get(response, "Auth").String()
				School := gjson.Get(response, "School").String()

				log.Printf("==>%s  学校: %s  院系: %s 上次登录时间 : %s 上次登录IP: %s \n",
					Name, School, Department, LastLogin, LastLoginIP)

				SessionId, _, err := GetSessionidAndSchoolNumber(int64(UserID), Auth)
				if err != nil {
					return
				}
				SaveCourseId(SessionId)
				// 当获取到SessionId后，发送到通道
				sessionIdChan <- SessionId
				errChan <- nil
				done <- struct{}{}
				return
			default:
				log.Printf("接收到未知消息 '%s': %v\n", op, response)
			}
		}
	}()

	select {
	case sessionId := <-sessionIdChan:
		return sessionId, <-errChan
	case <-time.After(1 * time.Minute):
		log.Println("等待时间过长，二维码已失效")
		return "", fmt.Errorf("等待时间过长，二维码已失效")
	}
}

func GetLessonId(sessionid, courseId string) string {
	client := &http.Client{}
	req, _ := http.NewRequest("GET", "https://www.yuketang.cn/api/v3/classroom/on-lesson-upcoming-exam", nil)
	req.AddCookie(&http.Cookie{Name: "sessionid", Value: sessionid})

	resp, err := client.Do(req)
	if err != nil {
		log.Println("Error:", err)
		time.Sleep(10 * time.Second)
		return GetLessonId(sessionid, courseId)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	jsonStr := string(body)

	// 使用gjson解析并查找lessonId
	lessons := gjson.Get(jsonStr, "data.onLessonClassrooms")
	var lessonId string
	lessons.ForEach(func(_, value gjson.Result) bool {
		if value.Get("courseId").String() == courseId {
			lessonId = value.Get("lessonId").String()
			return false // 找到后停止迭代
		}
		return true // 继续查找
	})

	if lessonId == "" {
		log.Println("---暂未找到课程对应的lessonId，请检查是否配置正确或确认是否在上课，程序将在10秒后重试---")
		time.Sleep(10 * time.Second)
		return GetLessonId(sessionid, courseId)
	}
	return lessonId
}

func GetLessonOtherInfo(sessionid string, lessonId string) (string, string, string) {
	url := "https://www.yuketang.cn/api/v3/lesson/checkin"
	jsonData := []byte(fmt.Sprintf(`{"source": 1, "lessonId": "%s"}`, lessonId))

	// 创建HTTP请求
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		log.Println("Error creating request:", err)
		return "", "", ""
	}

	// 设置Cookie
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "sessionid", Value: sessionid})

	// 发送HTTP请求
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println("Error making request:", err)
		return "", "", ""
	}
	defer resp.Body.Close()

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("Error reading response body:", err)
		return "", "", ""
	}

	// 使用gjson解析响应JSON
	lessonToken := gjson.GetBytes(body, "data.lessonToken").String()
	identityId := gjson.GetBytes(body, "data.identityId").String()
	auth := resp.Header.Get("Set-Auth")

	return lessonToken, identityId, auth
}

// UpdateBanks 函数功能：更新题库，参数：presentationID（string），返回值：无
func UpdateBanks(presentationID string, sessionid string, auth string) {
	// 读取 JSON 文件
	filePath := fmt.Sprintf("./ppts/%s.json", presentationID)
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		log.Println("Error reading file:", err)
		return
	}

	// 使用 gjson 解析文件
	slides := gjson.Get(string(fileData), "data.slides")

	var qaList []map[string]interface{}

	slides.ForEach(func(key, value gjson.Result) bool {
		// 如果 slide 中包含 problem
		if value.Get("problem").Exists() {
			problem := value.Get("problem")
			problemID := problem.Get("problemId").String()
			problemType := problem.Get("problemType").Int()
			question := problem.Get("body").String()

			// 获取 answers 或 result 字段
			answers := problem.Get("answers")
			if !answers.Exists() {
				answers = problem.Get("result")
			}

			// 处理 options
			options := map[string]string{}
			problem.Get("options").ForEach(func(_, option gjson.Result) bool {
				key := option.Get("key").String()
				value := option.Get("value").String()
				options[key] = value
				return true
			})

			teachLessonID := openLesson(sessionid, auth)

			qa := map[string]interface{}{
				"presentation_id": presentationID,
				"problemId":       problemID,
				"problemType":     problemType,
				"question":        question,
				"answers":         GetAnswer(problemID, teachLessonID, sessionid),
				"options":         options,
			}

			closeLesson(teachLessonID, sessionid)
			qaList = append(qaList, qa)
		}
		return true
	})

	// 读取已有的 bank.json 文件
	var existingQAList []map[string]interface{}
	bankFilePath := "./bank.json"
	bankFileData, err := os.ReadFile(bankFilePath)
	if err != nil {
		if os.IsNotExist(err) {
			existingQAList = []map[string]interface{}{}
		} else {
			log.Println("Error reading bank.json:", err)
			return
		}
	} else {
		if err := json.Unmarshal(bankFileData, &existingQAList); err != nil {
			log.Println("Error parsing bank.json:", err)
			return
		}
	}
	// 将原有的题目中所有presentation_id为当前presentationID的题目删除
	for i := 0; i < len(existingQAList); i++ {
		if existingQAList[i]["presentation_id"] == presentationID {
			existingQAList = append(existingQAList[:i], existingQAList[i+1:]...) // 删除当前presentationID的题目
			i--
		}
	}
	// 将新问题追加到已有列表中
	existingQAList = append(existingQAList, qaList...)

	// 将更新后的数据写回 bank.json
	updatedData, err := json.MarshalIndent(existingQAList, "", "    ")
	if err != nil {
		log.Println("Error marshaling data:", err)
		return
	}

	err = os.WriteFile(bankFilePath, updatedData, 0644)
	if err != nil {
		log.Println("Error writing to bank.json:", err)
		return
	}

	log.Println("--------------------------题库更新成功---------------------")
}

// 异步获取PPT
func StorePPT(sessionid, presentationID, auth string) error {
	// Create the request
	url := "https://www.yuketang.cn/api/v3/lesson/presentation/fetch"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	// Set query parameters
	query := req.URL.Query()
	query.Add("presentation_id", presentationID)
	req.URL.RawQuery = query.Encode()
	// Set headers
	req.Header.Set("Authorization", "Bearer "+auth)
	req.AddCookie(&http.Cookie{Name: "sessionid", Value: sessionid})

	// Make the request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("请求失败，状态码: %d，响应体: %s", resp.StatusCode, body)
		return fmt.Errorf("请求失败，状态码: %d", resp.StatusCode)
	}
	defer resp.Body.Close()

	// Read the response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	// log.Printf("响应体长度: %d", len(body))

	// Save the response to a file
	fileName := fmt.Sprintf("./ppts/%s.json", presentationID)
	err = os.WriteFile(fileName, body, 0644)
	if err != nil {
		return err
	}

	log.Printf("-----------PPT文件保存在: %s------------\n", fileName)
	UpdateBanks(presentationID, sessionid, auth) // 更新题库
	return nil
}

// 补偿提交问题答案
func CompensateSubmit(sessionid string, auth string, unlockedproblem []gjson.Result, problemDelay map[string]int) {
	log.Println("--------------答案提交补偿机制正在运行--------------")
	// 遍历unlockedproblem，提交所有未提交的问题
	for _, problem := range unlockedproblem {
		PostAnswer(sessionid, auth, problem.String(), problemDelay, 0)
	}
	log.Println("--------------答案提交补偿机制运行完成--------------")
}

// solveProblem 函数 功能：提交问题答案 参数： 返回值：
func PostAnswer(sessionid string, auth string, problemId string, delay map[string]int, signal int) {
	// 读取并解析bank.json文件
	bankData, err := os.ReadFile("./bank.json")
	if err != nil {
		log.Println("读取bank.json文件失败:", err)
		return
	}

	banks := gjson.Parse(string(bankData)).Array()
	var problemType int
	var answers string
	var question string

	for _, bank := range banks {
		if bank.Get("problemId").String() == problemId {
			problemType = int(bank.Get("problemType").Int())
			answers = bank.Get("answers").String()
			question = bank.Get("question").String()
			break
		}
	}

	// 构建请求数据
	jsonData := fmt.Sprintf(`{
		"problemId": "%s",
		"problemType": %d,
		"dt": %d,
		"result": ["%s"]
	}`, problemId, problemType, time.Now().UnixNano()/int64(time.Millisecond), answers)
	// 将answers中的换行符去掉，赋值给answer
	answer := strings.ReplaceAll(answers, "\n", "")
	// 将jsonData保存成文件
	// os.WriteFile("./problem.json", []byte(jsonData), 0644)
	// 发送POST请求
	if signal == 1 {
		// 取一个随机数
		// 判断大小值
		if delay["min"] > delay["max"] {
			delay["min"], delay["max"] = delay["max"], delay["min"]
			// 输出消息
			log.Println("-------------同志！！！共产主义接班人连一亿之内的数字都分不清大小是吧？？？？-------------")
		}
		randomNum := rand.Intn(delay["max"]-delay["min"]) + delay["min"]
		// 输出随机数
		log.Printf("---当前题目%s:%s，答案:%s==>将于%d秒后提交答案(配置的延时范围为%d~%d秒)---", problemId, question, answer, randomNum, delay["min"], delay["max"])
		time.Sleep(time.Duration(randomNum) * time.Second)
	}
	client := &http.Client{}
	req, err := http.NewRequest("POST", "https://www.yuketang.cn/api/v3/lesson/problem/answer", strings.NewReader(jsonData))
	if err != nil {
		log.Println("创建请求失败:", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+auth)
	req.Header.Set("Cookie", "sessionid="+sessionid)
	//req.AddCookie(&http.Cookie{Name: "sessionid", Value: sessionid})
	// 延时三秒
	// time.Sleep(3 * time.Second)
	resp, err := client.Do(req)
	if err != nil {
		log.Println("发送请求失败:", err)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("读取响应失败:", err)
		return
	}

	// 解析响应
	a := string(body)
	result := gjson.Parse(a)
	// log.Println(result.String())

	if result.Get("code").Int() == 0 && result.Get("msg").String() == "OK" {
		log.Printf("题目标号为%s的题目提交成功！！题目为%s，答案为%s\n", problemId, question, answer)
	} else {
		log.Printf("题目标号为%s的题目提交失败！！题目为%s，答案为%s\n", problemId, question, answer)
	}
}
func CheckSessionId(sessionid string) bool {
	urlStr := "https://www.yuketang.cn/api/v3/classroom/on-lesson-upcoming-exam"
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		log.Println("Error creating request:", err)
		return false
	}

	cookie := &http.Cookie{
		Name:  "sessionid",
		Value: sessionid,
	}
	req.AddCookie(cookie)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println("Error making request:", err)
		return false
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("Error reading response body:", err)
		return false
	}

	responseJson := string(bodyBytes)
	result := gjson.Get(responseJson, "code").Int() == 0 && gjson.Get(responseJson, "msg").Str == "OK"

	return result
}

// GetSessionidAndSchoolNumber 登录并获取sessionid，之后获取用户的schoolNumber
func GetSessionidAndSchoolNumber(userID int64, auth string) (string, int64, error) {
	// 创建一个新的HTTP客户端
	client := &http.Client{}

	// 设置登录URL
	loginURL := "https://www.yuketang.cn/pc/web_login"
	// 构造登录数据
	loginData := map[string]interface{}{
		"UserID": userID,
		"Auth":   auth,
	}

	// 将数据转换为JSON格式
	dataBytes, err := json.Marshal(loginData)
	if err != nil {
		return "", 0, fmt.Errorf("格式转化失败: %w", err)
	}

	// 创建一个带有JSON数据的POST请求
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, loginURL, bytes.NewBuffer(dataBytes))
	if err != nil {
		return "", 0, fmt.Errorf("请求头创建错误: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	// 检查响应状态码
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", 0, fmt.Errorf("登录失败! 状态码: %d, 响应体: %s", resp.StatusCode, body)
	}

	// 获取sessionid
	var sessionid string
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "sessionid" {
			sessionid = cookie.Value
		}
	}

	if sessionid == "" {
		return "", 0, errors.New("未在cookies中发现sessionid")
	}

	// 使用sessionid发起新的请求以获取用户信息
	userInfoURL := "https://www.yuketang.cn/api/v3/user/basic-info"
	req, err = http.NewRequestWithContext(context.Background(), http.MethodGet, userInfoURL, nil)
	if err != nil {
		return "", 0, fmt.Errorf("error creating request: %w", err)
	}
	req.Header.Set("Cookie", fmt.Sprintf("sessionid=%s", sessionid))

	// 发送请求
	resp, err = client.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("error making request: %w", err)
	}
	defer resp.Body.Close()

	bodyText, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, fmt.Errorf("error reading response body: %w", err)
	}

	// 解析schoolNumber
	schoolNumberStr := gjson.Get(string(bodyText), "data.schoolNumber").String()
	var schoolNumber int64
	if schoolNumberStr != "" {
		var err error
		schoolNumber, err = strconv.ParseInt(schoolNumberStr, 10, 64)
		if err != nil {
			return "", 0, fmt.Errorf("无法将schoolNumber从字符串转换为int64: %w", err)
		}
	} else {
		return "", 0, errors.New("未在响应中找到schoolNumber")
	}

	return sessionid, schoolNumber, nil
}

// 函数功能： 控制台显示输出二维码 ，参数: ticket(二维码链接) （string） ，返回值：无
func ShowQrcode(ticket string) {
	// 生成二维码
	qrcode, err := qrcode.New(ticket, qrcode.Low) // 使用传入的ticket
	if err != nil {
		log.Fatalf("生成二维码: %v", err)
	}
	// 将二维码转换为ASCII字符
	ascii := qrcode.ToSmallString(false)
	// 等待用户扫描二维码
	log.Println("------------------------请使用微信扫描二维码，然后长按图片-识别进入雨课堂-----------------------------")
	// 输出二维码
	fmt.Println(ascii)
}
func SaveCourseId(sessionid string) {
	// 创建请求
	client := &http.Client{}
	req, err := http.NewRequest("GET", "https://www.yuketang.cn/v2/api/web/courses/list", nil)
	if err != nil {
		log.Println("Error creating request:", err)
		return
	}

	// 设置参数和 Cookie
	query := req.URL.Query()
	query.Add("identity", "2")
	req.URL.RawQuery = query.Encode()
	req.Header.Add("Cookie", "sessionid="+sessionid)

	// 发送请求
	resp, err := client.Do(req)
	if err != nil {
		log.Println("Error making request:", err)
		return
	}
	defer resp.Body.Close()

	// 读取响应体
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println("Error reading response body:", err)
		return
	}

	// 打开文件准备写入
	file, err := os.Create("courseId.txt")
	if err != nil {
		log.Println("Error creating file:", err)
		return
	}
	defer file.Close()

	// 解析 JSON 并提取课程 ID 和名称
	courses := gjson.Get(string(body), "data.list")
	courses.ForEach(func(key, value gjson.Result) bool {
		courseID := value.Get("course.id").Int()
		courseName := value.Get("course.name").String()

		// 打印课程 ID 和名称
		// log.Printf("%d %s\n", courseID, courseName)

		// 写入文件
		file.WriteString(fmt.Sprintf("%d     %s\n", courseID, courseName))

		return true // 继续遍历
	})
}

func openLesson(sessionid string, auth string) string {
	//checkclassroomid
	classroomId := classroomcheck(sessionid, auth)

	data := map[string]interface{}{
		"classroomId": classroomId,
		"title":       "答案采样授课",
		"chapterId":   nil,
		"sectionId":   nil,
	}

	// 将数据编码成 JSON 格式
	jsonData, err := json.Marshal(data)
	if err != nil {
		fmt.Println("Error marshaling JSON:", err)
		return ""
	}

	// 创建 POST 请求
	url := "https://www.yuketang.cn/api/v3/lesson/add"
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		fmt.Println("Error creating request:", err)
		return ""
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+auth)
	req.Header.Set("Cookie", "sessionid="+sessionid)

	// 发送请求
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error sending request:", err)
		return ""
	}
	defer resp.Body.Close()

	setAuth := resp.Header.Get("set-auth")

	// 输出响应
	return "Bearer " + setAuth
}

func GetAnswer(problemid string, teachLessonID string, sessionid string) string {
	// 构造请求 URL 和参数
	baseURL := "https://www.yuketang.cn/api/v3/lesson/problem/choice-detail"
	params := url.Values{}
	params.Add("problem_id", problemid)

	// 完整的 URL 带参数
	finalURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())

	// 创建 GET 请求
	req, err := http.NewRequest("GET", finalURL, nil)
	if err != nil {
		return ""
	}

	// 设置请求头
	req.Header.Set("Authorization", teachLessonID)
	req.Header.Set("Cookie", "sessionid="+sessionid)

	// 发送请求
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK {
		// 读取响应体
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return ""
		}

		// 定义一个结构体来解析JSON
		type ResponseData struct {
			Code int    `json:"code"`
			Msg  string `json:"msg"`
			Data struct {
				Display      string        `json:"display"`
				Distribution []interface{} `json:"distribution"`
				Unfinished   []interface{} `json:"unfinished"`
			} `json:"data"`
		}

		// 解析JSON响应
		var responseData ResponseData
		err = json.Unmarshal(body, &responseData)
		if err != nil {
			fmt.Println("Error unmarshaling response JSON:", err)
			return ""
		}

		// 提取 display 字段的值
		return responseData.Data.Display
	} else {
		fmt.Println("Unexpected status code:", resp.StatusCode)
	}
	return ""
}

func closeLesson(teachLessonID string, sessionid string) {
	url := "https://www.yuketang.cn/api/v3/lesson/end"
	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		fmt.Println("Error creating request:", err)
		return
	}

	req.Header.Set("Authorization", teachLessonID)
	req.Header.Set("cookie", "sessionid="+sessionid)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	return
}

func classroomcheck(sessionid string, auth string) string {
	url := "https://www.yuketang.cn/api/v3/classroom/drop-down"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return ""
	}
	req.Header.Set("Cookie", "sessionid="+sessionid)
	req.Header.Set("Authorization", auth)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ""
	}
	if resp.StatusCode == http.StatusOK {
		type Classroom struct {
			ClassroomId               string `json:"classroomId"`
			ClassroomName             string `json:"classroomName"`
			LessonCompanionPermission int    `json:"lessonCompanionPermission"`
			Archived                  bool   `json:"archived"`
			Active                    bool   `json:"active"`
		}

		type Course struct {
			CourseId       string      `json:"courseId"`
			CourseName     string      `json:"courseName"`
			UniversityId   string      `json:"universityId"`
			UniversityLogo string      `json:"universityLogo"`
			Classrooms     []Classroom `json:"classrooms"`
			TeacherManage  bool        `json:"teacherManage"`
			Archived       bool        `json:"archived"`
			Active         bool        `json:"active"`
		}

		type ResponseData struct {
			Code int    `json:"code"`
			Msg  string `json:"msg"`
			Data struct {
				Courses []Course `json:"courses"`
			} `json:"data"`
		}

		// 解析 JSON 响应体
		var responseData ResponseData
		err = json.Unmarshal(body, &responseData)
		if err != nil {
			fmt.Println("Error unmarshaling response JSON:", err)
			return ""
		}

		// 遍历 courses，查找 courseName 为 "法郎" 的班级ID
		// 遍历 courses，查找 courseName 为 "法郎" 的第一个 classroomId
		for _, course := range responseData.Data.Courses {
			if course.CourseName == "法郎" {
				if len(course.Classrooms) > 0 {
					return course.Classrooms[0].ClassroomId
				} else {
					fmt.Println("No classrooms found for 法郎.")
				}
				break // 找到后不再继续循环
			}
		}
	} else {
		fmt.Println("Unexpected status code:", resp.StatusCode)
	}
	return ""
}
