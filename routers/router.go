package routers

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/astaxie/beego"
	"github.com/astaxie/beego/context"
	"github.com/go-openapi/loads"
	"github.com/go-xorm/xorm"

	"github.com/soopsio/mgmt/api/v1/models"
	apimodels "github.com/soopsio/mgmt/api/v1/models"
	"github.com/soopsio/mgmt/api/v1/restapi"
	"github.com/soopsio/mgmt/api/v1/restapi/operations"
	"github.com/soopsio/mgmt/controllers"
	"github.com/thoas/stats"
	"go.uber.org/zap"
	// "github.com/soopsio/mgmt/pipe/longpoll"
)

var (
	logger *zap.Logger
	orm    *xorm.Engine
)

func Init(o *xorm.Engine, l *zap.Logger) {
	orm = o
	logger = l
}

func init() {
	middleware := stats.New()
	beego.Router("/", &controllers.MainController{})
	// ansible api 的逻辑由 goswagger 实现
	swaggerSpec, err := loads.Analyzed(restapi.SwaggerJSON, "")
	if err != nil {
		logger.Error(err.Error())
	}
	api := operations.NewAnsibleTaskAPI(swaggerSpec)

	rapi := restapi.NewServer(api)
	rapi.ConfigureAPI()
	rapi.SetAPI(api)
	beego.SetStaticPath("/api/v1/static", "static")

	var ApiFilter = func(ctx *context.Context) {
		// logger.Println(ctx.Request.URL)
		// checkRequest(ctx.Request)
		if !checkRequest(ctx.Request) {
			// ctx.Abort(401, "非法请求")
			ctx.Output.SetStatus(401)

			ctx.Output.JSON(models.Error{
				Code:    401,
				Fields:  "",
				Message: "身份验证失败",
			}, false, false)
			// ctx.Output.Body([]byte("非法请求"))
		}
	}

	beego.InsertFilter("/api/v1/*.*", beego.BeforeRouter, ApiFilter)
	beego.Handler("/api/v1/*.*", middleware.Handler(rapi.GetHandler()))

	mux := http.NewServeMux()
	mux.HandleFunc("/stats", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		b, _ := json.Marshal(middleware.Data())
		w.Write(b)
	})

	beego.Handler("/stats", middleware.Handler(mux))

	// 长轮询接口初始化
	// longpollManager, _ := longpoll.GetLongpollManager()
	// beego.Handler("/api/v1/events", http.HandlerFunc(longpollManager.SubscriptionHandler))
	beego.Router("/api/v1/events", &controllers.TaskEventController{})
	// // 长轮询任务ID接口初始化(阅后即焚)
	// taskIDManager, _ := longpoll.GetTaskIDLongpollManager()
	// beego.Handler("/api/v1/taskids", http.HandlerFunc(taskIDManager.SubscriptionHandler))
	beego.Router("/api/v1/taskids", &controllers.TaskIDController{})

}

func checkRequest(req *http.Request) bool {
	return true
	// 日志接口不需要验证
	if req.URL.Path == "/api/v1/events" || req.URL.Path == "/api/v1/taskids" {
		return true
	}

	// 参数排序

	req.ParseForm()
	var keys []string
	for k := range req.Form {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	logger.Info("req form keys", zap.Strings("keys", keys))
	// url参数，不含公共参数
	urlargs := []string{}
	for _, k := range keys {
		sort.Strings(req.Form[k])
		for _, val := range req.Form[k] {
			// Signature参数只用作校验，不参与计算签名
			if k != "Signature" {
				urlargs = append(urlargs[:], k+"="+url.QueryEscape(val))
			}
		}
	}
	buf, _ := ioutil.ReadAll(req.Body)
	reqbody1 := ioutil.NopCloser(bytes.NewBuffer(buf))
	reqbody2 := ioutil.NopCloser(bytes.NewBuffer(buf))
	req.Body = reqbody2
	body, err := ioutil.ReadAll(reqbody1)
	if err != nil {
		panic(err)
	}
	logger.Info("requestInfo", zap.String("RequestURI", req.RequestURI), zap.ByteString("body", body))
	// 如果body不为空，则在最后追加body体然后再计算签名
	if len(body) > 0 {
		// StringToSign = StringToSign + url.QueryEscape("&") + url.QueryEscape(string(body))
		urlargs = append(urlargs[:], string(body))
	}
	StringToSign := req.Method + "&" + url.QueryEscape("/") + "&" + url.QueryEscape(strings.Join(urlargs, "&"))
	// StringToSign := "GET" + "&" + url.QueryEscape("/") + "&" + url.QueryEscape(strings.Join(urlargs, "&"))

	logger.Debug(StringToSign)
	//hmac ,use sha1

	keyids, ok := req.Form["AccessKeyId"]
	if ok != true {
		return false
	}
	keyid := keyids[0]
	accesskey := &apimodels.AccessKey{}
	var keysecret string
	if ok, err := orm.Cols("access_key_secret").Where("access_key_id = ?", keyid).Get(accesskey); err != nil {
		logger.Error(err.Error())
		return false
	} else if ok {
		// 找到匹配用户
		keysecret = accesskey.KeySecret
	} else {
		// 没有找到匹配用户
		return false
	}
	key := []byte(keysecret + "&")
	mac := hmac.New(sha1.New, key)
	mac.Write([]byte(StringToSign))

	if _, ok := req.Form["Signature"]; ok {
		return req.Form["Signature"][0] == base64.StdEncoding.EncodeToString(mac.Sum(nil))
	} else {
		return false
	}
	return true
}
