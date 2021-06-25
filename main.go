package main

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	log "github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
)

var (
	k8sClient *kubernetes.Clientset
)

func getEndpoints(namespace, name string) (*corev1.Endpoints, error) {
	l := log.WithFields(
		log.Fields{
			"action": "getEndpoints",
		},
	)
	l.Print("get endpoints")
	ctx := context.Background()
	ec := k8sClient.CoreV1().Endpoints(namespace)
	endpoints, err := ec.Get(
		ctx,
		name,
		metav1.GetOptions{},
	)
	if err != nil {
		l.Printf("error=%+v", err)
		return endpoints, err
	}
	return endpoints, nil
}

type ReqJob struct {
	URL          string
	Method       string
	Body         []byte
	ResponseBody []byte
}

func sendRequest(u string, m string, bd []byte) ([]byte, error) {
	var rd []byte
	l := log.WithFields(
		log.Fields{
			"action": "sendRequest",
		},
	)
	l.Printf("startRequest url=%v method=%v", u, m)
	c := &http.Client{}
	req, err := http.NewRequest(m, u, bytes.NewReader(bd))
	if err != nil {
		l.Printf("error=%+v", err)
		return rd, err
	}
	req.Host = os.Getenv("ENDPOINT_NAME")
	res, rerr := c.Do(req)
	if rerr != nil {
		l.Printf("error=%+v", rerr)
		return rd, err
	}
	defer res.Body.Close()
	rd, derr := ioutil.ReadAll(res.Body)
	if derr != nil {
		l.Printf("error=%+v", derr)
		return rd, derr
	}
	l.Printf("endRequest url=%v method=%v", u, m)
	return rd, nil
}

func sendRequestWorker(w <-chan *ReqJob, r chan<- *ReqJob) {
	l := log.WithFields(
		log.Fields{
			"action": "sendRequestWorker",
		},
	)
	l.Print("start worker")
	for j := range w {
		l.Printf("handle job %+v", j.URL)
		rd, e := sendRequest(j.URL, j.Method, j.Body)
		if e != nil {
			l.Printf("error=%+v", e)
			log.Println(e)
		}
		j.ResponseBody = rd
		r <- j
	}
	l.Print("stop worker")
}

func handler(w http.ResponseWriter, r *http.Request) {
	l := log.WithFields(
		log.Fields{
			"action": "handler",
		},
	)
	ns := os.Getenv("NAMESPACE_NAME")
	en := os.Getenv("ENDPOINT_NAME")
	if r.FormValue("namespace") != "" {
		ns = r.FormValue("namespace")
	}
	if r.FormValue("endpoint") != "" {
		en = r.FormValue("endpoint")
	}
	e, err := getEndpoints(ns, en)
	if err != nil {
		l.Printf("error=%+v", err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var eps []string
	for _, s := range e.Subsets {
		for _, a := range s.Addresses {
			for _, p := range s.Ports {
				eps = append(eps, "http://"+a.IP+":"+strconv.Itoa(int(p.Port)))
			}
		}
	}
	defer r.Body.Close()
	bd, berr := ioutil.ReadAll(r.Body)
	if berr != nil {
		l.Printf("error=%+v", berr)
		http.Error(w, berr.Error(), http.StatusBadRequest)
		return
	}
	wr := make(chan *ReqJob, len(eps))
	rr := make(chan *ReqJob, len(eps))
	for i := 0; i < 10; i++ {
		l.Print("create request worker")
		go sendRequestWorker(wr, rr)
	}
	up := r.URL.Path
	for _, v := range eps {
		j := &ReqJob{
			URL:    fmt.Sprintf("%s%s", v, up),
			Body:   bd,
			Method: r.Method,
		}
		l.Printf("createjob=%v", j.URL)
		wr <- j
	}
	close(wr)
	var lj *ReqJob
	for i := 0; i < len(eps); i++ {
		lj = <-rr
	}
	fmt.Fprint(w, string(lj.ResponseBody))
}

func createKubeClient() error {
	l := log.WithFields(
		log.Fields{
			"action": "createKubeClient",
		},
	)
	l.Print("create client")
	var kubeconfig string
	var err error
	if os.Getenv("KUBECONFIG") != "" {
		kubeconfig = os.Getenv("KUBECONFIG")
	} else if home := homedir.HomeDir(); home != "" {
		kubeconfig = filepath.Join(home, ".kube", "config")
	}
	var config *rest.Config
	// naïvely assume if no kubeconfig file that we are running in cluster
	if _, err := os.Stat(kubeconfig); os.IsNotExist(err) {
		config, err = rest.InClusterConfig()
		if err != nil {
			l.Printf("error=%+v", err)
			return err
		}
	} else {
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			l.Printf("error=%+v", err)
			return err
		}
	}
	k8sClient, err = kubernetes.NewForConfig(config)
	if err != nil {
		l.Printf("error=%+v", err)
		return err
	}
	return nil
}

func init() {
	e := createKubeClient()
	if e != nil {
		log.Fatal(e)
	}
}

func main() {
	http.HandleFunc("/", handler)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	http.ListenAndServe(":"+port, nil)
}
