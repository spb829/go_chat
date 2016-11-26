package main

import (
  "container/list"
  "log"
  "net/http"
  "time"

  "github.com/googollee/go-socket.io" // socket.io 패키지 사용
)

// 채팅 이벤트 구조체 정의
type Event struct {
    EvtType     string  // 이벤트 타입
    User        string  // 사용자 이름
    Timestamp   int // 시간 값
    Text        string  // 메시지 텍스트
}

// 구독 구조체 정의
type Subscription struct {
    // Subscription(구독)이란 채팅방에서 오고가는 메시지를 받는다는 뜻
    Archive []Event         // 지금까지 쌓인 이벤트를 저장할 슬라이스
    New     <-chan Event    // 새 이벤트가 생길 때마다 데이터를 받을 수 있도록 이벤트 채널 생성
    //  join    : 사용자가 채팅방에 들어왔을 때 이벤트
    //  leave   : 사용자가 채팅방에서 나갔을 때 이벤트
    //  message : 채팅 메시지 이벤트
}

//  이벤트 생성 함수
func NewEvent(evtType, user, msg string) Event {
    // Event 구조체 인스턴스를 생성하여 리턴 (현재 시간은 time.Now()Unix()함수를 사용)
    return Event{evtType, user, int(time.Now().Unix()), msg}
}

var(
    subscribe   = make(chan (chan<- Subscription), 10)  // 구독 채널
    // 새로운 사용자가 채팅방에 들어와쓸 때 Subscription 채널에 전달하는 채널.
    // 실제 채팅 이벤트는 Subscription 구조체가 처리
    // subscribe 채널은 사용자의 Event 채널을 구독자 목록에 추가할 때 사용
    // 채널 안에 채널을 정의한 경우로, 채널 변수 자체를 주고 받을 수 있다.
    // Subscription 채널은 보내기만 하므로 chan<-Subscription 으로 보내기 전용 채널 정의
    unsubscribe = make(chan (<-chan Event), 10)         // 구독 해지 채널
    // 사용자가 채팅방에서 나갔을 때 이벤트 채널을 전달하는 채널
    // 구독자 목록에서 Event 채널을 삭제할 때 사용
    // <-chan Event와 같이 받기 전용 채널로 정의
    publish     = make(chan Event, 10)                  // 이벤트 발행 채널
    // 이벤트를 발행하는 채널
    // publish 채널을 통해 전달된 이벤트는 구독자 목록의 모든 사용자에게 전달

    // 3개의 채널 모두 버퍼가 10개인 채널로 생성되었다.
    // 버퍼가 10개라 하더라도 10명 이상의 사용자, 10개 이상의 이벤트를 처리할 수 있다.
)

// 새로운 사용자가 들어왔을 때 이벤트를 구독할 함수
func Subscribe() Subscription{
    // Subscription 타입의 채널을 생성한 뒤 subscribe 채널에 전송하는 함수
    c := make(chan Subscription)    // 채널을 생성하여
    subscribe <- c                  // 구독 채널에 보냄
    // Chatroom 함수에서 지금까지 쌓인 ㅇ벤트와 이벤트를 받을 수 있는 Event 채널을 생성하고
    // Subscription 구조체를 c에 보내므로 return <-c는 생성된 Subscription 구조체가
    // 올 때까지 대기한 뒤 구조체를 꺼내서 리턴한다.
    return<-c
}

// 사용자가 나갔을 때 구독을 취소할 함수
func (s Subscription) Cancel(){
    // 이벤트 채널을 unsubscribe채널로 보냄
    unsubscribe <- s.New    // 구독 해지 채널에 보냄

    // 사용자가 나갔기 때문에 이벤트들은 필요 없어졌으므로 for 반복문을 통해서
    // s.New 채널에서 값을 모두 꺼낸다.
    // 무한 루프
    for {
        select {
        case _, ok := <-s.New:  // 채널에서 값을 모두 꺼냄
            if !ok{             // 값을 모두 꺼내면 함수를 빠져나옴
                return
            }
        default:
            return
        }
    }
}

// NewEvent 함수를 통하여 join, leave, message 이벤트를 생성하고 publish 채널에 보내는 함수들
// join, leave 이벤트는 단순히 사용자가 들어왔거나 나갔다는 것을 알려준다.

// 사용자가 들어왔을 때 이벤트 발행
func Join(user string){
    publish <- NewEvent("join", user, "")
}

// 사용자가 채팅 메시지를 보냈을 때 이벤트 발행
func Say(user, message string) {
    publish <- NewEvent("message", user, message)
}

// 사용자가 나갔을 때 이벤트 발행
func Leave(user string){
    publish <- NewEvent("leave", user, "")
}

//구독, 구독 해지, 발행된 이벤트를 처리하는 Chatroom 함수
func Chatroom(){
    archive := list.New()       // 지금까지 쌓인 이벤트를 저장할 연결 리스트
    subscribers:= list.New()    // 구독자 목록을 저장할 연결 리스트

    // for반복문을 통하여 case를 나누고 각 채널에서 처리할 내용을 작성한다.
    for{
        select{
        // subscribe 채널에 값이 들어왔을 때
        case c := <-subscribe:  // 새로운 사용자가 들어왔을 때
            var events []Event

            for e := archive.Front(); e != nil; e = e.Next(){
                //쌓인 이벤트가 있다면
                // events 슬라이스에 이벤트를 저장
                events = append(events, e.Value.(Event))
            }

            subscriber := make(chan Event, 10)  // 이벤트 채널 생성
            subscribers.PushBack(subscriber)    // 이벤트 채널을 구독자 목록에 추가

            c <- Subscription{events, subscriber}   // 구독 구조체 인스턴스를 생성하여 채널 c에 보냄

        // publish 채널에 값이 들어왔을 때
        case event := <-publish: // 새 이벤트가 발행되었을 때
            // 모든 사용자에게 이벤트 전달
            for e := subscribers.Front(); e != nil; e = e.Next(){
                // 구독자 목록에서 이벤트 채널을 꺼냄
                subscriber := e.Value.(chan Event)
                // 방금 받은 이벤트를 이벤트 채널에 보냄
                subscriber <- event
            }

            // 이벤트를 무한정 저장할 수 없으므로 가장 먼저 들어온 이벤트부터 하나씩 삭제
            // 저장된 이벤트 개수가 20개가 넘으면
            if archive.Len() >= 20{
                archive.Remove(archive.Front()) // 이벤트 삭제
            }
            archive.PushBack(event) // 현재 이벤트를 저장

        // unsubscribe 채널에 값이 들어왔을 때
        case c := <-unsubscribe:    // 사용자가 나갔을 때
            for e := subscribers.Front(); e != nil; e = e.Next(){
                // 구독자 목록에서 이벤트 채널을 꺼냄
                subscriber := e.Value.(chan Event)

                // 구독자 목록에 들어있는 이벤트와 채널 c가 같으면
                if subscriber == c{
                    subscribers.Remove(e)   // 구독자 목록에서 삭제
                    break
                }
            }
        }
    }
}

func main(){
    server, err := socketio.NewServer(nil)  // socket.io 초기화
                                            // nil로 모든 통신 방식을 사용
    if err != nil{
        log.Fatal(err)
    }

    go Chatroom()   // 채팅방을 처리할 함수를 고루틴으로 실행

    // 웹 브라우저에서 socket.io로 접속했을 때 실행할 콜백 설정
    server.On("connection", func(so socketio.Socket){
        // 웹 브라우저가 접속되면
        s := Subscribe()    // 구독 처리
        Join(so.Id())       // 사용자가 채팅방에 들어왔다는 이벤트 발행

        // 이제까지의 이벤트 = 이전 대화기록 및 접속기록
        // Emit 함수로 클라이언트에게 메시지를 보낼 수 있다.
        // 메시지 이름은 "event"로 설정하며 HTML의 JavaScript에서 socket.io 메시지를 받을 때 사용한다.
        for _, event := range s.Archive{
            // 지금까지 쌓인 이벤트를 웹 브라우저로 접속한 사용자에게 보냄
            so.Emit("event", event)
        }

        newMessages := make(chan string)

        // socket.io 인스턴스인 so의 On 함수를 이용하여 각 상황 마다 콜백 함수를 실행 시킴

        // 웹 브라우저에서 보내오는 채팅 메시지를 받을 수 있도록 콜백
        so.On("message", func(msg string) {
            // 채팅 문자열을 newMessages 채널에 전송
            newMessages <- msg
        })

        // 예제에서는 간단히 so.Id()를 사용하여 세션 아이디로 로그인 로그아웃을 구현했으나 실제 서비스시
        // 사용자 로그인/회원가입 기능을 구현해야 한다.

        // 웹 브라우저의 접속이 끊어졌을 때 콜백 설정
        so.On("disconnection", func(){
            Leave(so.Id())  // 사용자가 나갔다는 이벤트 발행
            s.Cancel()      // 이벤트 구독 취소
        })

        go func(){
            // 이제 새 이벤트가 발행되면 Subscription 구조체의 New 채널에 값이 들어온다.
            // Chatroom 함수의 case event := <- publish: 부분에서 값을 보낸다.
            // 그리고 newMessages 채널도 처리해준다.
            for{
                select{
                case event := <-s.New:      // 채널에 이벤트가 들어오면
                    so.Emit("event", event) // 이벤트 데이터를 웹 브라우저에 보냄
                case msg := <-newMessages:  // 웹 브라우저에서 채팅 메시지를 보내오면
                    Say(so.Id(), msg)       // 채팅 메시지 이벤트 발행
                }
            }
        }()
    })

    http.Handle("/socket.io/", server)  // /socket.io/ 경로는 socket.io 인스턴스가 처리하도록 설정

    // Handle함수에서 "/" 경로는 index.html파일을 보여줄 수 있도록 설정해야 한다.
    http.Handle("/", http.FileServer(http.Dir(".")))    // 현재 디렉터리를 파일 서버로 설정

    // 핸들러를 설정했으므로 두 번째 변수는 nil로 지정
    http.ListenAndServe(":80", nil)                     // 80번 포트에서 웹 서버 실행
}
