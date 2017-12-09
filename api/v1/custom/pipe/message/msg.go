package message

// Message redis中获取的实时消息
type Message struct {
	TaskID  string     `json:"task_id"`
	Type    string     `json:"type"` // log, childstatus, taskstatus
	Content MsgContent `json:"content"`
}

type MsgContent struct {
	Host      string `json:"host"`
	Msg       string `json:"msg"`
	TaskName  string `json:"task_name"`
	Sequnce   string `json:"sequnce"` // 扩展字段，以后可以做全局自增id
	TimeStamp string `json:"timestamp"`
	Module    string `json:"module"`
	Status    string `json:"status"` // PENDING,STARTED,RUNNING,RECEIVED,UNREACHABLE,SUCCESS,SUCCESSANDCHANGED,SUCCESSANDNOTCHANGED,FAILURE,STOPED,FINISHED,TIMEOUT
}
