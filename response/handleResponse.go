package handleResponse

import (
	"encoding/json"
	"net/http"
)

func HandleResponse(res http.ResponseWriter, obj interface{}, status int) {
	res.WriteHeader(status)
	jsonString, _ := json.Marshal(obj)
	res.Write(jsonString)
}
