# Go语言实践 - concurrency

# Introduction

一定要管理goroutine的生命周期,确定goroutine何时创建,何时销毁.否则就会出现goroutine的泄露.

# Goroutine

### 1.1 Processes and Threads

OS会为一个应用创建一个进程.一个应用程序就像是一个容器,这个容器为了资源而运行.这些资源包括内存地址空间、文件句柄、设备和线程.

线程是操作系统调度的一种执行路径，用于在处理器执行我们在函数中编写的代码。一个进程从一个线程开始，即主线程，当该线程终止时，进程终止。这是因为主线程是应用程序的原点。然后，主线程可以依次启动更多的线程，而这些线程可以启动更多的线程。

无论线程属于哪个进程，操作系统都会安排线程在可用处理器上运行。每个操作系统都有自己的算法来做出这些决定。

程序是从主线程开始运行的,如果主线程结束则整个程序就终止了.

### 1.2 Goroutines and Parallelism

Go 语言层面支持的 go 关键字，可以快速的让一个函数创建为 goroutine，我们可以认为 main 函数就是作为 goroutine 执行的。操作系统调度线程在可用处理器上运行，Go运行时调度 goroutines 在绑定到单个操作系统线程的逻辑处理器中运行(P)。即使使用这个单一的逻辑处理器和操作系统线程，也可以调度数十万 goroutine 以惊人的效率和性能并发运行。

Concurrency is not Parallelism.

**并发不意味着并行!**

并发不是并行。并行是指两个或多个线程同时在不同的处理器执行代码。如果将运行时配置为使用多个逻辑处理器，则调度程序将在这些逻辑处理器之间分配 goroutine，这将导致 goroutine 在不同的操作系统线程上运行。但是，要获得真正的并行性，您需要在具有多个物理处理器的计算机上运行程序。否则，goroutines 将针对单个物理处理器并发运行，即使 Go 运行时使用多个逻辑处理器。

goroutine绑定到线程上,然后挂载到逻辑处理器中执行.逻辑处理器和线程是绑定的.

[![Image text](https://github.com/rayallen20/Go-000/raw/main/Week03/img/%E7%BA%BF%E7%A8%8B%E3%80%81%E9%80%BB%E8%BE%91%E5%A4%84%E7%90%86%E5%99%A8%E4%B8%8Egoroutine%E7%9A%84%E5%85%B3%E7%B3%BB%E7%A4%BA%E6%84%8F%E5%9B%BE.jpg)](https://github.com/rayallen20/Go-000/blob/main/Week03/img/线程、逻辑处理器与goroutine的关系示意图.jpg)



### 1.3 Keep yourself busy or do the work yourself

```go
package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hello")
	})

	go func() {
		if err := http.ListenAndServe(":8081", nil); err != nil {
			log.Fatal(err)
		}
	}()

	// 空的select语句将永远阻塞
	select {}
}
```

假设多核状态下,则匿名函数会被放到某个核心上去执行,也就是具备了**并行**的能力.goroutine是由go runtime来调度的(用户态),线程是OS负责调度的(内核态).所以goroutine更轻量级.

这段代码中,本来`ListenAndServe`应该是会阻塞的(因为本质上它是一个死循环),因此就把这段代码放到别的核心上去执行了,这样main()函数就可以执行一些以其他逻辑了.可是main()函数退出了,整个程序就退出了,这样`ListenAndServe`也就退出了.为了避免这种情况,有人就使用了一个空的select来阻塞main()函数.

不鼓励这种做法.因为如果`ListenAndServe`,按照这个写法,main()函数是无法感知到的,也无法让main()函数执行完毕进而退出,只能由这个匿名函数自己去做退出的操作(`log.Fatal()`,该函数调用了`os.Exit()`).这样的退出方式会导致defer无法执行.

2个缺点:

1. main()函数无法感知自己开启的goroutine的生命周期
2. 直接退出导致defer无法执行

即使我们去掉`go`,改为串行,使得main()函数可以感知`ListenAndServe`的报错情况,依旧无法解决上述的第2个缺点:defer无法执行.

```go
package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hello")
	})

	if err := http.ListenAndServe(":8081", nil); err != nil {
		log.Fatal(err)
	}
}
```

问题:有没有办法既能安全退出又不影响main()函数执行的的方案呢?

"Keep yourself busy":如果main()函数只有一个监听的逻辑,那么让main()函数阻塞是没有问题的.也就是把"busy"的工作自己做了,而非委派出去给别的函数做.

"do the work yourself":让main()函数自己处理,比委派给其他函数并无法感知这个函数的执行情况,要好得多.

### 1.4 Never start a goroutine without knowning when it will stop

那么如果想要委派出去,该怎么办呢?

```go
package main

import (
	"fmt"
	"net/http"
	_ "net/http/pprof"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(resp http.ResponseWriter, req *http.Request) {
		fmt.Fprintln(resp, "Hello")
	})

	// debug
	go http.ListenAndServe("127.0.0.1:8082", http.DefaultServeMux)
	// app traffic
	http.ListenAndServe("0.0.0.0:8081", mux)
}
```

假设此时main()函数中又要监听8081端口,又要监听8082端口.假设此时8081端口用于处理业务,8082端口用于profiling.那么2个Listen都会阻塞,就只能开一个`go func`去执行其中1个Listen.

这段代码的问题:

1. 在main()函数中开启的goroutine,main()函数无法得知何时退出,何时结束 -> 需要一种机制使main()能够感知到
2. 不应该在不知道一个goroutine合适结束的前提下,开启一个goroutine

也就是说,这种goroutine的声明周期需要被管理起来.

所以通常在启动一个goroutine时,要问自己2个问题:

1. 这个goroutine何时终止?(被动感知)
2. 如何能控制这个goroutine结束?(主动触发)

此时如果8082端口挂掉了,main()函数无法感知,当需要诊断问题时,该端口的工作情况必然是不符合预期的.通常希望二者有一个挂掉,就直接整个进程退出.

那么,如何改进呢?

```go
package main

import (
	"fmt"
	"net/http"
)

func serveApp() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(resp http.ResponseWriter, req *http.Request) {
		fmt.Fprintln(resp, "Hello")
	})
	http.ListenAndServe("127.0.0.1:8081", mux)
}

func serveDebug() {
	http.ListenAndServe("127.0.0.1:8082", http.DefaultServeMux)
}

func main() {
	go serveDebug()
	serveApp()
}
```

首先将这2个操作单独抽离出来.然后在main()函数中调用.那么如果`serveApp()`返回了,则main.main将返回,进而导致程序关闭.那么这种情况只能靠supervisor等进程管理工具重新将这个程序启动起来.如果`serveDebug()`出错,则main()仍旧感知不到.所以还需要继续改进

```go
package main

import (
	"fmt"
	"log"
	"net/http"
)

func serveApp() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(resp http.ResponseWriter, req *http.Request) {
		fmt.Fprintln(resp, "Hello")
	})
	if err := http.ListenAndServe("127.0.0.1:8081", mux); err != nil {
		log.Fatal(err)
	}
}

func serveDebug() {
	if err := http.ListenAndServe("127.0.0.1:8082", http.DefaultServeMux); err != nil {
		log.Fatal(err)
	}
}

func main() {
	go serveApp()
	go serveDebug()
	select {}
}
```

利用`log.Fatal()`内部调用`os.Exit()`的方式,来实现任何一个监听出错,则整个进程退出的功能.但还是没能解决上文中说到的另一个问题:defer不执行的问题没有解决.

**`log.Fatal()`只能在`main.main`中或`init`函数中使用!**

再举一个不好的启动监听写法:

```go
package main

import (
	"fmt"
	"log"
	"net/http"
)

func serveApp() {
	go func() {
		mux := http.NewServeMux()
		mux.HandleFunc("/", func(resp http.ResponseWriter, req *http.Request) {
			fmt.Fprintln(resp, "Hello")
		})
		if err := http.ListenAndServe("127.0.0.1:8081", mux); err != nil {
			log.Fatal(err)
		}
	}()
}

func serveDebug() {
	go func() {
		if err := http.ListenAndServe("127.0.0.1:8082", http.DefaultServeMux); err != nil {
			log.Fatal(err)
		}
	}()
}

func main() {
	serveApp()
	serveDebug()
	select {}
}
```

这样写的问题在于:如果不去读`serveApp()`和`serveDebug()`的代码,是不可能知道后台启了goroutine来做监听的!

回看1.1,"Keep yourself busy","busy work"应该是自己做的.也就是说"阻塞"的任务应该交给`serveApp()`,而开启goroutine应该是由main()函数去做的.

**启goroutine的一定是调用者!一定是调用者来决定是否将某个操作后台异步执行**

**对于函数的提供者而言,提供者不该假设行为.所谓的"假设行为",即不该自己内部起一个goroutine然后帮助函数的调用者将work消化掉,这样做就是"假设我的调用者要求我在后台执行某操作"**

继续改进.不要因为走了太远而忘记为何出发.记住我们的目的是:让main()函数能够感知goroutine的报错情况并能根据报错情况控制main()函数所开启的goroutine的生命周期.

```go
package main

import (
	"context"
	"fmt"
	"net/http"
)

func main() {
	done := make(chan error, 2)
	stop := make(chan struct{})

	go func() {
		done <- serveDebug(stop)
	}()

	go func() {
		done <- serveApp(stop)
	}()

	var stopped bool
	for i := 0; i < cap(done); i++ {
		if err := <- done; err != nil {
			fmt.Printf("error: %v", err)
		}

		if !stopped {
			stopped = true
			close(stop)
		}
	}
}

func serve(addr string, handler http.Handler, stop <-chan struct{}) error {
	s := http.Server{
		Addr: addr,
		Handler: handler,
	}

	go func() {
		<- stop
		// TODO: 此处的error应该wrap
		s.Shutdown(context.Background())
	}()

	return s.ListenAndServe()
}

func serveApp(stop <-chan struct{}) error {
	addr := "127.0.0.1:8081"
	handler := func(resp http.ResponseWriter, req *http.Request) {
		fmt.Fprintln(resp, "Hello")
	}

	return serve(addr, http.HandlerFunc(handler), stop)
}

func serveDebug(stop <-chan struct{}) error {
	addr := "127.0.0.1:8082"
	handler := http.DefaultServeMux

	return serve(addr, handler, stop)
}
```

注:空struct表示zero size.即0大小

首先,还是让调用者(也就是main()函数)来决定是否启动goroutine.管道stop用来广播信号.管道done用来存储2个goroutine的error.

控制生命周期:当main()函数感知到这2个goroutine中的任何一个有错误(也就是`err := <- done`不再阻塞)时,for循环将可以继续.由于bool类型的默认值为false,所以第1次循环(`cap(done)`为2,所以这个循环是可以循环2次的)时管道就会被关闭.管道关闭时会广播所有阻塞的goroutine,这样`serve()`函数在自己开启的goroutine内将不再阻塞,进而能够调用到`http.Server.Shutdown()`,实现平滑关闭.同时也达到了由main()函数来控制goroutine生命周期的目标.

感知错误:当`serveApp()`或`serveDebug()`二者任何一个报错时,将会向管道done中存入一个error,因此main()函数中能够从该管道中取出元素时,也就意味着main()函数感知到错误了.这样也就实现了感知报错情况的目标.

for循环2次的目的:实际上循环1次时stop就已经被关闭了.但是由于2个goroutine分别都要关闭,假设`serveApp()`是由于报错而关闭的,那么`serveDebug()`也要受该原因影响而关闭,因此`serveDebug()`的阻塞也要被取消,进而调用到`http.Server.Shutdown()`.则`serveDebug()`的`http.Server.ListenAndServer()`才能够退出.这样就实现了二者均平滑退出的目的.当然此时可能`serveDedebug()`返回的是nil.

// TODO: stop的类型是 `chan struct{}`,但`serve`的形参列表要求的是`<-chan struct{}`,为什么二者可以类型相同?

1. 对于一个函数而言,应该让其调用者来决定,该函数需要在前台执行还是后台执行
2. 调用者一定要能够感知并控制自己开启的goroutine的生命周期

再来看一个例子:

```go
// leak is a buggy function.It launches a goroutine that
// blocks receiving from a channel. Nothing will ever be sent
// on that channel and the channel is never closed so that goroutine
// will be blocked forever.
// leak 是一个有bug的函数.leak启动了一个goroutine,该goroutine会因为从管道中接收数据而阻塞.
// 不会有任何数据被发送至这个管道,而且这个管道不会被关闭.因此该goroutine将会一直被阻塞.
func leak() {
	ch := make(chan int)

	go func() {
		val := <- ch
		fmt.Println("We received a value, ", val)
	}()
}
```

leak()中的goroutine是会泄露的.因为永远不会有信号传递到`ch`中的,所以在`leak()`执行完毕后,这个goroutine一定会泄露的.因此不要写这种代码.

再来看一个例子:

```go
package main

import (
	"fmt"
	"log"
	"time"
)

func main() {
	err := process("abc")
	if err != nil {
		log.Print("err:", err)
	}
}

// search simulates a function that finds a record based
// on a search term. It takes 200ms to perform this work
// search 函数模仿了一个基于关键字查找记录的操作.此处用延迟200ms来表示这个操作.
func search(term string) (string, error) {
	time.Sleep(200 * time.Millisecond)
	return "some value", nil
}

// process is the work for the program. It finds a record
// then prints it
// process 是这段程序的运行者.该函数寻找一条记录并打印该记录.
func process(term string) error {
	record, err := search(term)
	if err != nil {
		return err
	}

	fmt.Println("Received:", record)
	return nil
}
```

这段代码的问题在于:对于`process()`函数而言,无法得知`search()`函数何时返回.所以需要做超时控制.

改进:使用context做超时控制

```go
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"
)

func main() {
	err := process("abc")
	if err != nil {
		log.Println("err:", err)
	}
}

// result wraps the return values from search. It allows us
// to pass both values across a single channel.
// result 封装了 search 的返回值.该结构体可以让我们把返回值通过一个管道进行传递
type result struct {
	record string
	err error
}

// search simulates a function that finds a record based
// on a search term. It takes 200ms to perform this work
// search 函数模仿了一个基于关键字查找记录的操作.此处用延迟200ms来表示这个操作.
func search(term string) (string, error) {
	time.Sleep(200 * time.Millisecond)
	return "some value", nil
}

// process is the work for the program. It finds a record
// then prints it. It fails if it takes more than 100ms.
// process 是这段程序的运行者.该函数寻找一条记录并打印该记录.
// 该函数在运行超过100ms的情况下会失败.
func process(term string) error {
	// Create a context that will be canceled in 100ms
	// 创建1个context,该context在100ms后被取消
	ctx, cancel := context.WithTimeout(context.Background(), 100 * time.Millisecond)
	defer cancel()

	// Make a channel for the goroutine to report its result
	// 为goroutine创建一个管道,该管道用于报告goroutine的工作结果
	ch := make(chan result)

	// Launch a goroutine to find the record. Create a result
	// from the returned values to send through the channel.
	// 开启一个goroutine用于寻找记录.根据该goroutine的返回值创建一个 result 的实例,
	// 然后发送将该实例发送至管道
	go func() {
		record, err := search(term)
		ch <- result{record, err}
	}()

	// Block waiting to either receive from the goroutine's
	// channel or for the context to be canceled.
	// 因为等待从goroutine的channel中接收结果或因为context被取消而阻塞
	select {
	case <- ctx.Done():
		return errors.New("search canceled")
	case result := <- ch:
		if result.err != nil {
			return result.err
		}
		fmt.Println("Received:", result.record)
		return nil
	}
}
```

1. 后台执行交给调用者
2. 要有一种机制知道自己创建的goroutine何时退出
3. 要做超时控制

### 1.5 Leave concurrency to the caller

Question1: 以下2个API有何区别?

```go
// ListDirectoryA returns the contents of dir.
// ListDirectoryA 返回dir下的内容
func ListDirectoryA(dir string) ([]string, error) {
	directories := make([]string, 10, 10)
	return directories, nil
}

// ListDirectoryB returns a channel over which directory entries
// will be published. When the list of entries is exhausted,
// the channel will be closed.
// ListDirectoryB 返回一个管道,dir下的目录项将会被推送至这个管道中.
// 当dir下的目录项全部被推送至管道时,管道将会被关闭
func ListDirectoryB(dir string) chan string {
	directories := make(chan string, 10)
	go func() {
		for {
			directories <- "mock dir"
		}
	}()
	return directories
}
```

对于`ListDirectoryA()`来讲,若dir下的目录树非常庞大,需要枚举非常久,就需要等待很长时间.因此就会想到:是不是可以让其返回一个channel,对于调用者而言就可以不断地从该channel中读取目录.

那么实际上`ListDirectoryB()`是存在2个问题的:

1. 必须通过关闭channel的方式,才能告知调用者:目录树读取完毕.如果读取目录树的过程中报错了,其调用者是无法得知这个channel到底是读取目录树中途报错而被关闭,还是目录树读取完毕而被关闭.(这个或许用结构体包一下,加一个error还是可解的)
2. 调用者必须持续的从这个管道中读取,直到这个管道被关闭.假设调用者只需要目录树中的某一个值出现,之后就不再需要从该管道中读取了.此时对于该函数的调用者而言,将不再消费该channel,进而会导致`ListDirectoryB()`中负责读取目录树的goroutine在把路径存入该channel时,发生阻塞.为了避免这个问题,调用者必须持续消费该管道,直到该管道被关闭.最终可能并没有比返回slice更快.

那么如何改进呢?

可以参考标准库filepath.Walk()函数的实现.建议传入一个callback.枚举dir时,执行callback.通过callback来控制ListDirectory的行为.

```go
func ListDirectoryC(dir string, fn func(string2 string)) {
	
}
```

https://pkg.go.dev/path/filepath#Walk

对于main()函数调用filepath.Walk()时传入的callback来讲.其形参列表中的error表示`filepath.Walk()`解析目录时报错了,这样可以在callback内部控制退出,不再让`filepath.Walk()`继续枚举了.其次就是自己的业务逻辑想要控制`filepath.Walk()`退出(比如找到了感兴趣的目录).

**应该把并发在函数内部自己消化掉,而非是交给调用者来处理.**

```go
package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
)

func prepareTestDirTree(tree string) (string, error) {
	tmpDir, err := ioutil.TempDir("", "")
	if err != nil {
		return "", fmt.Errorf("error creating temp directory: %v\n", err)
	}

	err = os.MkdirAll(filepath.Join(tmpDir, tree), 0755)
	if err != nil {
		os.RemoveAll(tmpDir)
		return "", err
	}

	return tmpDir, nil
}

func main() {
	tmpDir, err := prepareTestDirTree("dir/to/walk/skip")
	if err != nil {
		fmt.Printf("unable to create test dir tree: %v\n", err)
		return
	}
	defer os.RemoveAll(tmpDir)
	os.Chdir(tmpDir)

	subDirToSkip := "skip"

	fmt.Println("On Unix:")
	err = filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Printf("prevent panic by handling failure accessing a path %q: %v\n", path, err)
			return err
		}
		if info.IsDir() && info.Name() == subDirToSkip {
			fmt.Printf("skipping a dir without errors: %+v \n", info.Name())
			return filepath.SkipDir
		}
		fmt.Printf("visited file or dir: %q\n", path)
		return nil
	})
	if err != nil {
		fmt.Printf("error walking the path %q: %v\n", tmpDir, err)
		return
	}
}
```

### 1.6 Incomplete Work

goroutine泄露的例子:

```go
package main

import (
	"log"
	"net/http"
	"time"
)

// Tracker knows how to track events for the application
// Tracker 知道如何追踪APP的事件
type Tracker struct {}

// Event records an event to a database or stream.
// Event 记录一个事件到数据库或字节流
func (t *Tracker) Event(data string) {
	// Simulate network write latency.
	// 模拟网络写入延迟
	time.Sleep(time.Millisecond)
	log.Println(data)
}

// App holds application state
// App 表示应用的状态
type App struct {
	track Tracker
}

// Handle represents an example handler for the web service.
// Handle 表示一个web服务处理的示例
func(a *App) Handle(w http.ResponseWriter, r *http.Request) {
	// Do some actual work.
	// 做一些业务逻辑处理

	// Respond to the client
	// 响应客户端
	w.WriteHeader(http.StatusCreated)

	// Fire and Hope
	// TODO: 这句不会翻译
	// BUG: We are not managing this goroutine
	// BUG: 我们没有管理这个goroutine
	go a.track.Event("this event")
}

func main() {
	var app App
	http.HandleFunc("/", app.Handle)
	http.ListenAndServe("localhost:8081", nil)
}
```

`Tracker`对象用于记录一些埋点操作.`App`对象表示应用.

像本例中,虽然创建goroutine的工作交给了调用者,但是这种1个请求就开1个goroutine的做法依旧不是很好(当然这不是此处要讲的重点).同时由于埋点操作是一个旁路的操作,所以应该是放在后台去做的.可是问题在于:

1. 在`App.Handle()`中启动的goroutine是没有被管理起来的!**不要写这种代码,因为调用者无法知道goroutine中的函数何时退出.**
2. 在不知道还有多少个goroutine在运行的情况下,无法平滑退出.只能kill -9了

改进:使用`sync.WaitGroup`来确保goroutine的运行结束

```go
package main

import (
	"log"
	"net/http"
	"sync"
	"time"
)

// Tracker knows how to track events for the application
// Tracker 知道如何追踪APP的事件
type Tracker struct {
	wg sync.WaitGroup
}

// Event starts tracking an event. It runs asynchronously to
// not block the caller. Be sure to call the Shutdown function
// before the program exits so all tracked events finish.
// Event 记录一个事件.为了不让调用者造成阻塞,该函数以异步的方式运行.
// 在程序退出前请确保调用 Shutdown 方法,其目的在于所有 Event 的goroutine都能退出
func (t *Tracker) Event(data string) {
	// Increment counter so Shutdown knows to wait for this event
	// 计数器+1,为了让 Shutdown 方法感知到等待该goroutine结束
	t.wg.Add(1)

	// Track event in a goroutine so caller is not blocked.
	// 为了不让调用者阻塞,追踪事件的操作放在一个goroutine中去做
	go func() {
		// Decrement counter to tell Shutdown this goroutine finished
		// 计数器-1,以便告知 Shutdown 方法:该goroutine执行结束了
		defer t.wg.Done()

		// Simulate network write latency.
		// 模拟网络写入延迟
		time.Sleep(time.Millisecond)
		log.Println(data)
	}()
}

// Shutdown waits for all tracked events to finish processing.
// Shutdown 方法等待所有追踪事件的goroutine结束,以便能够平滑退出进程
func (t *Tracker) Shutdown() {
	t.wg.Wait()
}

// App holds application state
// App 表示应用的状态
type App struct {
	track Tracker
}

// Handle represents an example handler for the web service.
// Handle 表示一个web服务处理的示例
func(a *App) Handle(w http.ResponseWriter, r *http.Request) {
	// Do some actual work.
	// 做一些业务逻辑处理

	// Respond to the client
	// 响应客户端
	w.WriteHeader(http.StatusCreated)

	// Track the event
	a.track.Event("this event")
}

func main() {
	// Start a server
	// 启动服务
	// Details not shown...
	// 细节略过
	var app App

	// Shut the server down
	// 停止服务
	// Details not shown...
	// 细节略过

	// Wait for all event goroutines to finish
	// 等待所有追踪事件的goroutine结束
	app.track.Shutdown()
}
```

在`Tracker.Shutdown()`方法中,`sync.WaitGroup.Wait()`方法会阻塞.直到计数器的值为0时,不再阻塞.在main()函数中,调用`Tracker.Shutdown()`,即可实现平滑退出.保证追踪事件不会丢失.这样就知道了goroutine的退出时间.

可是依旧没有解决的一个问题是:这样做还是1个请求进来就开了1个goroutine.而且虽然知道了goroutine的退出时间,但如果`Event()`执行时间过长,我们在退出时,还是要等待`app.track.Shutdown()`的.所以我们还需要一个超时控制的机制.

继续改进.

```go
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"sync"
	"time"
)

// Wait up to 5 seconds for all event goroutines to finish
// 为了等待所有goroutine结束,等待5s
const timeout = 5 * time.Second

// Tracker knows how to track events for the application
// Tracker 知道如何追踪APP的事件
type Tracker struct {
	wg sync.WaitGroup
}

// Event starts tracking an event. It runs asynchronously to
// not block the caller. Be sure to call the Shutdown function
// before the program exits so all tracked events finish.
// Event 记录一个事件.为了不让调用者造成阻塞,该函数以异步的方式运行.
// 在程序退出前请确保调用 Shutdown 方法,其目的在于所有 Event 的goroutine都能退出
func (t *Tracker) Event(data string) {
	// Increment counter so Shutdown knows to wait for this event
	// 计数器+1,为了让 Shutdown 方法感知到等待该goroutine结束
	t.wg.Add(1)

	// Track event in a goroutine so caller is not blocked.
	// 为了不让调用者阻塞,追踪事件的操作放在一个goroutine中去做
	go func() {
		// Decrement counter to tell Shutdown this goroutine finished
		// 计数器-1,以便告知 Shutdown 方法:该goroutine执行结束了
		defer t.wg.Done()

		// Simulate network write latency.
		// 模拟网络写入延迟
		time.Sleep(time.Millisecond)
		log.Println(data)
	}()
}

// Shutdown waits for all tracked events to finish processing
// or for the provided context to be canceled
// Shutdown 方法等待所有追踪事件的goroutine结束,以便能够平滑退出进程
// 或者提供一个context用于退出
func (t *Tracker) Shutdown(ctx context.Context) error {
	// Create a channel to signal when the wait group is finished.
	// 创建一个管道,当wait group结束时,发送信号
	ch := make(chan struct{})

	// Create a goroutine to wait for all other goroutines to be
	// done then close the channel to unblock to select.
	// 创建一个goroutine用于等待所有追踪事件的goroutine结束,然后关闭管道.
	// 关闭管道的作用在于解除select代码块的阻塞
	go func() {
		t.wg.Wait()
		close(ch)
	}()

	// Block this function from returning. Wait for either the
	// wait group to finish or the context to expire.
	// 在返回前阻塞本函数.等待wait group结束或context到期
	select {
	case <- ch:
		return nil
	case <- ctx.Done():
		return errors.New("timeout")
	}
}

// App holds application state
// App 表示应用的状态
type App struct {
	track Tracker
}

// Handle represents an example handler for the web service.
// Handle 表示一个web服务处理的示例
func(a *App) Handle(w http.ResponseWriter, r *http.Request) {
	// Do some actual work.
	// 做一些业务逻辑处理

	// Respond to the client
	// 响应客户端
	w.WriteHeader(http.StatusCreated)

	// Track the event
	// 追踪事件
	a.track.Event("this event")
}

func main() {
	// Start a server
	// 启动服务
	// Details not shown...
	// 细节略过
	var app App

	// Shut the server down
	// 停止服务
	// Details not shown...
	// 细节略过

	// Wait for all event goroutines to finish
	// 等待所有追踪事件的goroutine结束
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	err := app.track.Shutdown(ctx)
	if err != nil {
		log.Println(err)
	}
}
```

将`wg.Wait()`的操作托管给另一个goroutine.实际上这里如果超时了,还是强退了.但总算是解决了超时控制的问题.可核心的问题没有解决:这样做还是1个请求进来就开1个goroutine的(`Event()`中开启goroutine的问题没有解决).继续改进.

// TODO:这个"1个请求进来就开1个goroutine"的解释可能不太对,还得琢磨琢磨

```go
package main

import (
	"context"
	"fmt"
	"time"
)

func main() {
	tr := NewTracker()
	go tr.Run()
	_ = tr.Event(context.Background(), "test")
	_ = tr.Event(context.Background(), "test")
	_ = tr.Event(context.Background(), "test")
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(2 * time.Second))
	defer cancel()
	tr.Shutdown(ctx)
}

func NewTracker() *Tracker {
	return &Tracker{
		ch: make(chan string, 10),
	}
}

type Tracker struct {
	// 用于存储事件的管道
	ch chan string
	// 用于标识消费事件操作结束的管道
	stop chan struct{}
}

func(t *Tracker) Event(ctx context.Context, data string) error {
	select {
	case t.ch <- data:
		return nil
	case <- ctx.Done():
			return ctx.Err()
	}
}

func(t *Tracker) Run() {
	for data := range t.ch {
		time.Sleep(1 * time.Second)
		fmt.Println(data)
	}
	// t.ch关闭后,跳出for range 向管道中放入一个空结构体
	// 标识运行Run()方法的goroutine可以被结束了
	t.stop <- struct{}{}
}

func(t *Tracker) Shutdown(ctx context.Context) {
	// 关闭存储事件的管道,以便 Run 方法能够发送信号至stop管道
	close(t.ch)
	select {
	case <- t.stop:
	case <- ctx.Done():
	}
}
```

`Event()`由于传入了context,现在生命周期可以被管控了;当管道`ch`被关闭后,`Run()`方法的遍历就可以结束了,然后就可以向`stop`管道发送信号,表示运行该方法的goroutine可以被关闭了.这样由main()函数创建的,用于执行`Run()`方法的goroutine,其生命周期就可以交由main()函数管理了.

## 

### 管住 Goroutine 的生命周期

### Keep yourself busy or do the work yourself.

在main goroutine退出后，所有的程序都会退出,所以为了阻塞main goroutine，有时候会做点骚操作！

```go
func main() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Hello, GopherCon SG")
	})
	go func() {
		// 是否会出错，main goroutine 感知不到，也处理不了。
		if err := http.ListenAndServe(":8080", nil); err != nil {
			log.Fatal(err) // Fatal() 底层调用了 os.Exit()当报错直接退出
		}
	}()
	select { // 永远阻塞。
	}
}
```

- ❌ `go` 一个 goroutine 去 `ListenAndServe`，main 使用 `select{}` 阻塞。

main goroutine 会阻塞，无法处理别的事情，即使 `ListenAndServe` 的 goroutine 出了错， 它也不会得知，也无法处理，两个 goroutine 之间缺少通讯机制。

### Never start a goroutine without knowing when it will stop

当启动一个 goroutine 时，要明确两个问题：

- 它什么时候会结束（terminate）？
- 它要怎样结束，要达到什么样的条件，怎么让它退出？

查看下列案例： 尝试在两个不同的端口上提供 http 流量：8080 用于应用程序流量；8081 用于访问 /debug/pprof 端点。

```go
func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(resp http.ResponseWriter, req *http.Request) {
		fmt.Fprintln(resp, "Hello, QCon!")
	})
	// ↓ 如果不使用 go，会阻塞在这一行，再下一行的 ListenAndServe 就没有机会执行
	//   但这里启动后不管了，这种做法是不好的，应该管理 goroutine 的结束。启动者要对 goroutine 的生命周期负责。
	go http.ListenAndServe("127.0.0.1:0801", http.DefaultServeMux)
	http.ListenAndServe("0.0.0.0:8080", mux)
}
```

这个例子有什么问题呢？

- 启动的 goroutine 是否成功、出错，主 goroutine 完全无法得知，
- 主 goroutine 也因用于监听服务阻塞，没有能力处理其他事务。

把处理流程写在主函数也太丑了吧！我们把两个提出来，然后再用`go`出去，再想办法阻塞主函数：

```go
func serveApp() {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(resp http.ResponseWriter, req *http.Request) {
		fmt.Fprintln(resp, "Hello, QCon!")
	})
	if err := http.ListenAndServe("0.0.0.0:8080", mux); err != nil {
		log.Fatal(err)
	}
}

func serveDebug() {
	if err := http.ListenAndServe("127.0.0.1:8001", http.DefaultServeMux); err != nil {
		log.Fatal(err)
	}
}

func main() {
	go serveDebug()
	go serveApp()
	select {}
}
```

这个看起来好像简洁了很多了，但是问题来了是不是跟第一个差不多，都是没有出错的处理而且还犯了一点：Only use log.Fatal from main.main or init functions

我们期望使用一种方式，同时启动业务端口和 debug 端口，如果任一监听服务出错，应用都退出。 此时，当当当当，channel 闪亮登场！！！！！！

```go
func serve(addr string, handler http.Handler, stop <-chan struct{}) error {
	s := http.Server{
		Addr:    addr,
		Handler: handler,
	}
	go func() {
		<-stop // wait for stop signal
		s.Shutdown(context.Background())
	}()
	return s.ListenAndServe()
}
func serveApp(stop <-chan struct{}) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(resp http.ResponseWriter, req *http.Request) {
		fmt.Fprintln(resp, "Hello, QCon!")
	})
	return serve("0.0.0.0:8080", mux, stop)
}
func serveDebug(stop <-chan struct{}) error {
	return serve("127.0.0.1:8081", http.DefaultServeMux, stop)
}

func main() {
	done := make(chan error, 2)
	stop := make(chan struct{})
	go func() {
		done <- serveDebug(stop)
	}()
	go func() {
		done <- serveApp(stop)
	}()
	// serveApp、serveDebug 任意一个出错，都会解除 <-done的阻塞
	// close(stop) 会广播解除所有 <-stop 的阻塞，没有出错的监听也会被 shutdown
	var stopped bool
	for i := 0; i < cap(done); i++ { // 循环两次是为了等所有的 server 平滑安全退出
		if err := <-done; err != nil {
			fmt.Printf("error: %v\n", err)
		}
		if !stopped {
			stopped = true
			close(stop)
		}
	}

}
```

如果这个时候再有一个 goroutine 可以向 stop 传入一个 struct{}，就可以控制整个进程平滑停止。这里可以参见[go-workgroup](https://github.com/da440dil/go-workgroup)

但是呢？在使用chan的时候记得有发有接，没有发送端会导致需要接收chan数据的goroutine一直被阻塞。

那对于一些需要超时控制的呢？也不能让它串行运行吧？使用 context.WithTimeout() 实现超时控制 。所以看下列代码：

```go
// search模拟一个基于搜索词查找记录的函数。完成这项工作需要200毫秒。
func search(term string) (string, error) {
	time.Sleep(200 * time.Millisecond)
	return "some value", nil
}

// process是程序的工作。它找到一条记录，然后打印它。
func process(term string) error {
	record, err := search(term)
	if err != nil {
		return err
	}
	fmt.Println("Received:", record)
	return nil
}

// result 包装来自搜索的返回值。它允许我们通过单个通道传递这两个值
type result struct {
	record string
	err    error
}

// processWithTimeout 。它找到一条记录，然后打印它，如果花费的时间超过100ms，就会失败。
func processWithTimeout(term string) error {

	// 创建一个将在100ms内取消的上下文
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	//为goroutine创建一个通道来报告其结果。
	ch := make(chan result)

	// 启动goroutine来查找记录。创建一个结果从返回值通过通道发送。
	go func() {
		record, err := search(term)
		ch <- result{record, err}
	}()

	// 等待从goroutine的通道接收或等待上下文被取消的阻塞
	select {
	case <-ctx.Done():
		return errors.New("search canceled")
	case result := <-ch:
		if result.err != nil {
			return result.err
		}
		fmt.Println("Received:", result.record)
		return nil
	}
}

func main() {
	err := process("without-timeout")
	if err != nil {
		fmt.Printf("process: error: %v\n", err)
	}
	err = processWithTimeout("with-timeout")
	if err != nil {
		fmt.Printf("processWithTimeout: error: %v\n", err)
	}
}
```

#### 将并发留给调用者使用

```go
// ListDirectory returns the contents of dir.
func ListDirectory(dir string) ([]string, error)

// ListDirectory returns a channel over which
// directory entries will be published. When the list
// of entries is exhausted, the channel will be closed.
func ListDirectory(dir string) chan string
```

这两个API：

- 将目录读取到一个 slice 中，然后返回整个切片，或者如果出现错误，则返回错误。 这是同步调用的，ListDirectory 的调用方**会阻塞，直到读取所有目录条目**。 根据目录的大小，这**可能需要很长时间**，并且**可能会分配大量内存**来构建目录条目名称的 slice。
- ListDirectory 返回一个 chan string，将通过该 chan 传递目录。 当通道关闭时，这表示不再有目录。 由于在 ListDirectory 返回后发生通道的填充，ListDirectory 可能内部启动 goroutine 来填充通道。 这个版本有两个问题：
  - 通过使用一个关闭的通道作为不再需要处理的项目的信号， ListDirectory 无法告诉调用者通过通道返回的项目集不完整，因为中途遇到了错误。 调用方无法区分空目录与完全从目录读取的错误之间的区别。 这两种方法（读完或出错）都会导致从 ListDirectory 返回的通道会立即关闭。
  - 调用者必须持续从通道读取，直到它关闭， 因为这是调用者知道开始填充通道的 goroutine 已经停止的唯一方法。 这对 ListDirectory 的使用是一个严重的限制，调用者必须花时间从通道读取数据， 即使它可能已经收到了它想要的答案。 对于大中型目录，它可能在内存使用方面更为高效，但这种方法并不比原始的基于 slice 的方法快。

更好的 API：

```go
func ListDirectory(dir string, fn func(string))
```

`filepath.Walk`也是类似的模型。 如果函数启动 goroutine，则必须向调用方提供显式停止该goroutine 的方法。 通常，将异步执行函数的决定权交给该函数的调用方通常更容易。

## 总结

总结一下这一部分讲到的几个要点，这也是我们

1. 请将是否异步调用的选择权交给调用者，不然很有可能大家并不知道你在这个函数里面使用了 goroutine
2. 如果你要启动一个 goroutine 请对它负责（控制其生命周期）
3. 永远不要启动一个你无法控制它退出，或者你无法知道它何时推出的 goroutine
4. 还有上一篇提到的，启动 goroutine 时请加上 panic recovery 机制，避免服务直接不可用
5. 造成 goroutine 泄漏的主要原因就是 goroutine 中造成了阻塞，并且没有外部手段控制它退出
6. 尽量避免在请求中直接启动 goroutine 来处理问题，而应该通过启动 worker 来进行消费，这样可以避免由于请求量过大，而导致大量创建 goroutine 从而导致 oom，当然如果请求量本身非常小，那当我没说。

# Memory model

1. Happend Before：在一个 goroutine 中，读和写一定是按照程序中的顺序执行的。即编译器和处理器只有在不会改变这个 goroutine 的行为时才可能修改读和写的执行顺序。由于重排（CPU 重排、编译器重排），不同的goroutine 可能会看到不同的执行顺序。
2. 多个 goroutine 访问共享变量 v 时，它们必须使用同步事件来建立先行发生这一条件来保证读操作能看到需要的写操作。
   1. 对变量v的零值初始化在内存模型中表现的与写操作相同。
   2. 对大于 single machine word 的变量的读写操作表现的像以不确定顺序对多个 single machine word的变量的操作。
   3. 参考 https://www.jianshu.com/p/5e44168f47a3
3. 因此，多个goroutine要确保没有data race 需要保证：原子性（以singel machine word操作或者利用同步语义）、可见性（消除内存屏障）
4. 关于底层的 memory reordering，可以挖一挖 cpu cacline、锁总线、mesi、memory barrier。
5. 编译器重排、内存重排，(重排是指程序在实际运行时对内存的访问顺序和代码编写时的顺序不一致)，目的都是为了减少程序指令数，最大化提高CPU利用率。
6. Memory Barrier: 现代 CPU 为了“抚平”内核、内存、硬盘之间的速度差异，搞出了各种策略，例如三级缓存等。每个线程的操作结果可能先缓存在自己内核的L1 L2cache，此时（还没刷到内存）别的内核线程是看不到的。因此，对于多线程的程序，所有的 CPU 都会提供“锁”支持，称之为 barrier，或者 fence。它要求：barrier 指令要求所有对内存的操作都必须要“扩散”到 memory 之后才能继续执行其他对 memory 的操作。 参考 https://cch123.github.io/ooo/，https://blog.csdn.net/qcrao/article/details/92759907



# Package sync

Go 的并发原语 goroutines 和 channels 为构造并发软件提供了一种优雅而独特的方法。

在Go中如果我们写完代码想要对代码是否存在数据竞争进行检查，可以通过`go build -race` 对程序进行编译

```go
package main

import (
	"fmt"
	"sync"
)

var Wait sync.WaitGroup
var Counter int = 0

func main() {
	for routine := 1; routine <= 2; routine++ {
		Wait.Add(1)
		go Routine()
	}
	Wait.Wait()
	fmt.Printf("Final Counter:%d\n", Counter)
}

func Routine() {
	Counter++
	Wait.Done()
}
```

`go build -race `编译后的程序，运行可以很方便看到代码中存在的问题

```
==================
WARNING: DATA RACE
Read at 0x000001277ce0 by goroutine 8:
  main.Routine()
      /Users/zhaofan/open_source_study/test_code/202012/race/main.go:21 +0x3e

Previous write at 0x000001277ce0 by goroutine 7:
  main.Routine()
      /Users/zhaofan/open_source_study/test_code/202012/race/main.go:21 +0x5a

Goroutine 8 (running) created at:
  main.main()
      /Users/zhaofan/open_source_study/test_code/202012/race/main.go:14 +0x6b

Goroutine 7 (finished) created at:
  main.main()
      /Users/zhaofan/open_source_study/test_code/202012/race/main.go:14 +0x6b
==================
Final Counter:2
Found 1 data race(s)
```

对于锁的使用： 最晚加锁，最早释放。

对于下面这段代码，这是模拟一个读多写少的情况，正常情况下，每次读到cfg中的数字都应该是依次递增加1的，但是如果运行代码，则会发现，会出现意外的情况。

```go
package main

import (
	"fmt"
	"sync"
)

var wg sync.WaitGroup

type Config struct {
	a []int
}

func main() {
	cfg := &Config{}
	// 这里模拟数据的变化
	go func() {
		i := 0
		for {
			i++
			cfg.a = []int{i, i + 1, i + 2, i + 3, i + 4, i + 5}
		}
	}()

	// 这里模拟去获取数据
	var wg sync.WaitGroup
	for n := 0; n < 4; n++ {
		wg.Add(1)
		go func() {
			for n := 0; n < 20; n++ {
				fmt.Printf("%v\n", cfg)
			}
			wg.Done()
		}()
	}
	wg.Wait()
}
```

对于上面这个代码的解决办法有很多

- Mutex
- RWMutext
- Atomic

对于这种读多写少的情况，使用RWMutext或Atomic 都可以解决，这里只写写一个两者的对比，通过测试也很容易看到两者的性能差别：

```go
package main

import (
	"sync"
	"sync/atomic"
	"testing"
)

type Config struct {
	a []int
}

func (c *Config) T() {

}

func BenchmarkAtomic(b *testing.B) {
	var v atomic.Value
	v.Store(&Config{})

	go func() {
		i := 0
		for {
			i++
			cfg := &Config{a: []int{i, i + 1, i + 2, i + 3, i + 4, i + 5}}
			v.Store(cfg)
		}
	}()

	var wg sync.WaitGroup
	for n := 0; n < 4; n++ {
		wg.Add(1)
		go func() {
			for n := 0; n < b.N; n++ {
				cfg := v.Load().(*Config)
				cfg.T()
				// fmt.Printf("%v\n", cfg)
			}
			wg.Done()
		}()
	}
	wg.Wait()
}

func BenchmarkMutex(b *testing.B) {
	var l sync.RWMutex
	var cfg *Config

	go func() {
		i := 0
		for {
			i++
			l.RLock()
			cfg = &Config{a: []int{i, i + 1, i + 2, i + 3, i + 4, i + 5}}
			cfg.T()
			l.RUnlock()
		}
	}()

	var wg sync.WaitGroup
	for n := 0; n < 4; n++ {
		wg.Add(1)
		go func() {
			for n := 0; n < b.N; n++ {
				l.RLock()
				cfg.T()
				l.RUnlock()
			}
			wg.Done()
		}()
	}
	wg.Wait()
}
```

从结果来看性能差别还是非常明显的：

```
 zhaofan@zhaofandeMBP  ~/open_source_study/test_code/202012/atomic_ex2  go test -bench=. config_test.go
goos: darwin
goarch: amd64
BenchmarkAtomic-4       310045898                3.91 ns/op
BenchmarkMutex-4        11382775               101 ns/op
PASS
ok      command-line-arguments  3.931s
 zhaofan@zhaofandeMBP  ~/open_source_study/test_code/202012/atomic_ex2  
```

Mutext锁的实现有一下几种模式：

- Barging, 这种模式是为了提高吞吐量，当锁释放时，它会唤醒第一个等待者，然后把锁给第一个等待者或者第一个请求锁的人。注意这个时候释放锁的那个goroutine 是不会保证下一个人一定能拿到锁，可以理解为只是告诉等待的那个人，我已经释放锁了，快去抢吧。
- Handsoff，当释放锁的时候，锁会一直持有直到第一个等待者准备好获取锁，它降低了吞吐量，因为锁被持有，即使另外一个goroutine准备获取它。相对Barging，这种在释放锁的时候回问下一个要获取锁的，你准备好了么，准备好了我就把锁给你了。
- Spinning，自旋在等待队列为空或者应用程序重度使用锁时效果不错，parking和unparking goroutines 有不低的性能成本开销，相比自旋来说要慢的多。

Go 1.8 使用了Bargin和Spinning的结合实现。当试图获取已经被持有的锁时，如果本地队列为空并且P的数量大于1，goroutine 将自旋几次（用一个P旋转会阻塞程序），自旋后，goroutine park 在程序高频使用锁的情况下，它充当了一个快速路径。

Go1.9 通过添加一个新的饥饿模式来解决出现锁饥饿的情况，该模式将会在释放的时候触发handsoff, 所有等待锁超过一毫秒的goroutine（也被称为有界等待）将被诊断为饥饿，当被标记为饥饿状态时，unlock方法会handsoff把锁直接扔给第一个等待者。

在饥饿模式下，自旋也会被停用，因为传入的goroutines将没有机会获取为下一个等待者保留的锁。

#### errgroup

https://pkg.go.dev/golang.org/x/sync/errgroup

使用场景，如果我们有一个复杂的任务，需要拆分为三个任务goroutine 去执行，errgroup 是一个非常不错的选择。

下面是官网的一个例子：

```go
package main

import (
	"fmt"
	"golang.org/x/sync/errgroup"
	"net/http"
)

func main() {
	g := new(errgroup.Group)
	var urls = []string{
		"http://www.golang.org/",
		"http://www.google.com/",
		"http://www.somestupidname.com/",
	}
	for _, url := range urls {
		// Launch a goroutine to fetch the URL.
		url := url // https://golang.org/doc/faq#closures_and_goroutines
		g.Go(func() error {
			// Fetch the URL.
			resp, err := http.Get(url)
			if err == nil {
				resp.Body.Close()
			}
			return err
		})
	}
	// Wait for all HTTP fetches to complete.
	if err := g.Wait(); err == nil {
		fmt.Println("Successfully fetched all URLs.")
	}
}
```

#### Sync.Poll

sync.poll的场景是用来保存和复用临时对象，减少内存分配，降低GC压力， Request-Drive 特别适合

Get 返回Pool中的任意一个对象，如果Pool 为空，则调用New返回一个新创建的对象

放进pool中的对象，不确定什么时候就会被回收掉，如果实现Put进去100个对象，下次Get的时候发现Pool是空的也是有可能的。所以sync.Pool中是不能放连接型的对象。所以sync.Pool中应该放的是任意时刻都可以被回收的对象。

sync.Pool中的这个清理过程是在每次垃圾回收之前做的，之前每次GC是都会清空pool, 而在1.13版本中引入了victim cache, 会将pool内数据拷贝一份，避免GC将其清空，即使没有引用的内容也可以保留最多两轮GC。



### Package context

在Go 服务中，每个传入的请求都在自己的goroutine中处理，请求处理程序通常启动额外的goroutine 来访问其他后端，如数据库和RPC服务，处理请求的goroutine通常需要访问特定于请求（request-specific context）的值,例如最终用户的身份，授权令牌和请求的截止日期。**当一个请求被取消或者超时时，处理该请求的所有goroutine都应该快速推出，这样系统就可以回收他们正在使用的任何资源。*

如何将context 集成到API中？

- 首参数传递context对象
- 在第一个request对象中携带一个可选的context对象

**注意：尽量把context 放到函数的首选参数，而不要把context 放到一个结构体中。**

### context.WithValue

为了实现不断WithValue, 构建新的context，内部在查找key时候，使用递归方式不断寻找匹配的key，知道root context(Backgrond和TODO value的函数会返回nil)

context.WithValue 方法允许上下文携带请求范围的数据，这些数据必须是安全的，以便多个goroutine同时使用。这里的数据，更多是面向请求的元数据，而不应该作为函数的可选参数来使用(比如context里挂了一个sql.Tx对象，传递到Dao层使用)，因为元数据相对函数参数更多是隐含的，面向请求的。而参数更多是显示的。 同一个context对象可以传递给在不同的goroutine中运行的函数；上下文对于多个goroutine同时使用是安全的。对于值类型最容易犯错的地方，在于context value 应该是不可修改的，每次重新赋值应该是新的context，即： `context.WithValue(ctx, oldvalue)`，所以这里就是一个麻烦的地方，如果有多个key/value ，就需要多次调用`context.WithValue`， 为了解决这个问题，https://pkg.go.dev/google.golang.org/grpc/metadata 在grpc源码中使用了一个metadata.

`func FromIncomingContext(ctx context.Context) (md MD, ok bool)` 这里的md 就是一个map `type MD map[string][]string` 这样对于多个key/value的时候就可以用这个MD 一次把多个对象挂进去，不过这里需要注意：如果一个groutine从ctx中读出这个map对象是不能直接修改的。因为如果这个时候ctx被传递给了多个gouroutine， 如果直接修改就会导致data race, 因此需要使用copy-on-write的思路，解决跨多个goroutine使用数据，修改数据的场景。

比如如下场景：

新建一个context.Background() 的ctx1, 携带了一个map 的数据， map中包含了k1:v1 的键值对，ctx1 作为参数传递给了两个goroutine,其中一个goroutine从ctx1中获取map1，构建一个新的map对象map2,复制所有map1的数据，同时追加新的数据k2:v2 键值对，使用context.WithValue 创建新的ctx2,ctx2 会继续传递到其他groutine中。 这样各自读取的副本都是自己的数据，写行为追加的数据在ctx2中也能完整的读取到，同时不会污染ctx1中的数据，这种处理方式就是典型的COW(COPY ON Write)

### context cancel

当一个context被取消时， 从它派生的所有context也将被取消。WithCancel(ctx)参数认为是parent ctx， 在内部会进行一个传播关系链的关联。Done() 返回一个chan，当我们取消某个parent context， 实际上会递归层层cancel掉自己的chaild context 的done chan 从而让整个调用链中所有监听cancel的goroutine退出

下面是官网的例子,稍微调整了一下代码：

```go
package main

import (
	"context"
	"fmt"
)

func main() {
	// gen generates integers in a separate goroutine and
	// sends them to the returned channel.
	// The callers of gen need to cancel the context once
	// they are done consuming generated integers not to leak
	// the internal goroutine started by gen.
	gen := func(ctx context.Context) <-chan int {
		dst := make(chan int)
		n := 1
		go func() {
			for {
				select {
				case <-ctx.Done():
					return // returning not to leak the goroutine
				case dst <- n:
					n++
				}
			}
		}()
		return dst
	}

	ctx, cancel := context.WithCancel(context.Background())

	for n := range gen(ctx) {
		fmt.Println(n)
		if n == 5 {
			cancel()
		}
	}
}
```

如果实现一个超时控制，通过上面的context的parent/child 机制， 其实只需要启动一个定时器，然后再超时的时候，直接将当前的context给cancel掉，就可以实现监听在当前和下层的context.Done()和goroutine的退出。

```go
package main

import (
	"context"
	"fmt"
	"time"
)

const shortDuration = 1 * time.Millisecond

func main() {
	d := time.Now().Add(shortDuration)
	ctx, cancel := context.WithDeadline(context.Background(), d)

	// Even though ctx will be expired, it is good practice to call its
	// cancellation function in any case. Failure to do so may keep the
	// context and its parent alive longer than necessary.
	defer cancel()

	select {
	case <-time.After(1 * time.Second):
		fmt.Println("overslept")
	case <-ctx.Done():
		fmt.Println(ctx.Err())
	}

}
```

关于context 使用的规则总结：

- Incoming requests to a server should create a Context.
- Outgoing calls to servers should accept a Context.
- Do not store Contexts inside a struct type; instead, pass a Context explicitly to each function that needs it.
- The chain of function calls between them must propagate the Context.
- Replace a Context using WithCancel, WithDeadline, WithTimeout, or WithValue.
- When a Context is canceled, all Contexts derived from it are also canceled.
- The same Context may be passed to functions running in different goroutines; Contexts are safe for simultaneous use by multiple goroutines.
- Do not pass a nil Context, even if a function permits it. Pass a TODO context if you are unsure about which Context to use.
- Use context values only for request-scoped data that transits processes and APIs, not for passing optional parameters to functions.
- All blocking/long operations should be cancelable.
- Context.Value obscures your program’s flow.
- Context.Value should inform, not control.
- Try not to use context.Value.





# chan

#### Unbuffered Channels

```
ch := make(chan struct{})
```

无缓冲 chan 没有容量，因此进行任何交换前需要两个 goroutine 同时准备好。 当 goroutine 试图将一个资源发送到一个无缓冲的通道并且没有goroutine 等待接收该资源时， 该通道将锁住发送 goroutine 并使其等待。 当 goroutine 尝试从无缓冲通道接收，并且没有 goroutine 等待发送资源时， 该通道将锁住接收 goroutine 并使其等待。

**无缓冲信道的本质是保证同步。**

[demo19](https://github.com/dowenliu-xyz/Go-000/blob/main/Week03/cmd/demo19/demo19.go)

- Receive 先于 Send 完成。
- 好处：100% 保证能收到。
- 代价：延迟时间未知

#### Buffered Channels

buffered channel 具有容量，因此其行为可能有点不同。 当 goroutine 试图将资源发送到缓冲通道， 而该通道已满时， 该通道将锁住 goroutine并使其等待缓冲区可用； 如果通道中有空间，发送可以立即进行，goroutine 可以继续。 当goroutine 试图从缓冲通道接收数据，而缓冲通道为空时， 该通道将锁住 goroutine 并使其等待资源被发送。

> 在 chan 创建过程中定义的缓冲区大小可能会极大地影响性能。 chan 锁住和解锁 goroutine 时，goroutine parking/unparking 会比较耗时， goroutine 上下文切换消耗比较多。

- Send 先于 Receive 发生。
- 好处: 延迟更小。
- 代价: 不保证数据到达，越大的 buffer，越小的保障到达。buffer = 1 时，给你延迟一个消息的保障。

#### Design Philosophy 设计理念

- If any given Send on a channel CAN cause the sending goroutine to block:
  当向一个 channel 发送时，允许发送 goroutine 阻塞：

  - Not allowed to use a Buffered channel larger than 1.

    不要使用缓冲区大小超过1的有缓冲 channel

    - Buffers larger than 1 must have reason/measurements. 设置缓冲区大小1必须要有充分的理由或压测。

  - Must know what happens when the sending goroutine blocks.
    必须明确知道发送 goroutine 阻塞的原因。

- If any given Send on a channel WON'T cause the sending goroutine to block:
  当向一个 channel 发送时，不想让发送 goroutine 阻塞：

  - You have the exact number of buffers for each send.

    如果你要发送的内容数量与缓冲区大小刚好匹配。

    - Fan Out pattern 使用扇出模式。

  - You have the buffer measured for max capacity.

    如果你要发送的内容数量超出 channel 的最大容量

    - Drop pattern 放弃部分数据，使用丢弃模式

      ```
      select {
      case ch<-data:
      default:
      }
      ```

- Less is more with buffers. 通道缓冲越少越好。

  - Don't think about performance when thinking about buffers.
    缓冲区大小与性能无关

  - Buffers can help to reduce blocking latency between signaling.

    缓冲区大小只与阻塞延迟有关

    - Reducing blocking latency towards zero does not necessarily mean better throughput.
      阻塞延迟与吞吐量无关。吞吐与消费 channel 的 goroutine 数量有关。
    - If a buffer of one is giving you good enough throughput then keep it.
    - Question buffers that are larger than one and measure for size. 想要将缓冲区大小设置为超过1时，不要想当然，要通过压测确定缓冲区大小。
    - Find the smallest buffer possible that provides good enough throughput.
      在保证足够好的吞吐量的前提下，缓冲区大小要尽量小。

#### Go Concurrency Patterns

- Timing out 超时处理

  ```
  select {
  case data1<-ch1:
      // if recieved
  case ch2<-data2:
      // if sent
  case <-time.After(duration):
      // if timeout
  ```

- Moving on 放弃数据

  - Drop pattern

- Pipeline

- Fan-out, Fan-in

- Cancellation

  - Close 等于 Receive 发生（类似 Buffered）。
  - 不需要传递数据，或者传递 nil
  - 非常适合去做超时控制

- Context

> https://blog.golang.org/concurrency-timeouts
> https://blog.golang.org/pipelines
> https://talks.golang.org/2013/advconc.slide#1
> https://github.com/go-kratos/kratos/tree/master/pkg/sync

> **一定由Sender关闭channel**



# References

https://medium.com/@cep21/how-to-correctly-use-context-context-in-go-1-7-8f2c0fafdf39