package response

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"

	"github.com/go-playground/validator"
)

func WriteJson(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func WriteError(w http.ResponseWriter, status int, message interface{}) {
	errKey := "error"
	if reflect.TypeOf(message).Kind() == reflect.Map {
		errKey = "errors"
	}
	WriteJson(w, status, map[string]interface{}{errKey: message})
}

func WriteValidationErrors(w http.ResponseWriter, errors error, args interface{}) {
	valErrors := errors.(validator.ValidationErrors)
	errRes := make(map[string]string)
	for _, err := range valErrors {
		fieldName := err.Field()
		field, _ := reflect.TypeOf(args).FieldByName(fieldName)
		fmt.Println(field)
		fieldJSONName, _ := field.Tag.Lookup("json")

		switch err.ActualTag() {
		case "required":
			errRes[fieldJSONName] = fmt.Sprintf("%s is required", fieldJSONName)
		case "gt":
			errRes[fieldJSONName] = fmt.Sprintf("%s must be greater than %s", fieldJSONName, err.Param())
		default:
			errRes[fieldJSONName] = fmt.Sprintf("%s is not valid", fieldJSONName)
		}
	}
	WriteError(w, http.StatusBadRequest, errRes)
}