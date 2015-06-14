package jsgrpc

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"reflect"
	"sync"

	"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

func Invoke(ctx context.Context, method string, args, reply proto.Message, cc *grpc.ClientConn, opts ...grpc.CallOption) error {
	panic("Not implemented.")
}

type Server struct {
	mu  sync.Mutex
	m   map[string]*service // service name -> service info
	mux *http.ServeMux
}

func NewServer() *Server {
	return &Server{
		m:   make(map[string]*service),
		mux: http.NewServeMux(),
	}
}

// service consists of the information of the server serving this service and
// the methods in this service.
type service struct {
	server interface{} // the server for service methods
	md     map[string]*grpc.MethodDesc
	//	sd     map[string]*StreamDesc
}

func handler(service interface{}, md *grpc.MethodDesc) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Service method call: %s", md.MethodName)
		data, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if len(data) == 0 {
			data = []byte("{}")
		}
		ret, err := md.Handler(service, nil, data)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		data, err = json.Marshal(ret)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		i, err := w.Write(data)
		if err != nil {
			log.Printf("During write: %s", err)
		} else if l := len(data); i != l {
			log.Printf("Wrote %d bytes, want %d", i, l)
		}
	}
}

// RegisterService register a service and its implementation to the gRPC
// server. Called from the IDL generated code. This must be called before
// invoking Serve.
func (s *Server) RegisterService(sd *grpc.ServiceDesc, ss interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Does some sanity checks.
	if _, ok := s.m[sd.ServiceName]; ok {
		log.Fatalf("grpc: Server.RegisterService found duplicate service registration for %q", sd.ServiceName)
	}
	ht := reflect.TypeOf(sd.HandlerType).Elem()
	st := reflect.TypeOf(ss)
	if !st.Implements(ht) {
		log.Fatalf("grpc: Server.RegisterService found the handler of type %v that does not satisfy %v", st, ht)
	}
	srv := &service{
		server: ss,
		md:     make(map[string]*grpc.MethodDesc),
		//		sd:     make(map[string]*StreamDesc),
	}
	for i := range sd.Methods {
		d := &sd.Methods[i]
		srv.md[d.MethodName] = d
		name := fmt.Sprintf("/%s/%s", sd.ServiceName, d.MethodName)
		log.Printf("Register: %s", name)
		s.mux.HandleFunc(name, handler(ss, d))
	}
	//	for i := range sd.Streams {
	//		d := &sd.Streams[i]
	//		srv.sd[d.StreamName] = d
	//	}
	s.m[sd.ServiceName] = srv
}

// implements http.Handler interface
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("RPC server call: %s", r.URL.Path)
	s.mux.ServeHTTP(w, r)
}
