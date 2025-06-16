package utils

import (
	"fmt"
	"os"
	"strings"

	v1 "github.com/crossplane/crossplane-runtime/apis/common/v1"
	"github.com/fatih/color"
	"github.com/google/uuid"
	"gopkg.in/alecthomas/kingpin.v2"
)

var (
	fieldColor = color.New(color.FgHiGreen).SprintFunc()
	attributeColor = color.New(color.FgHiMagenta).SprintFunc()
)

var (
	errOpenFile = "Could not open file"
	errCreateCredEnvironment = "Not able to create CredEnvironment"
	errCreateScanner = "Could not create scanner"
	errGetTransactionID = "TRANSACTION_ID not found"

)

func OpenFile(path string) *os.File {
	file, err := os.Open(path)
	if os.IsNotExist(err) {
		file, _ = os.Create(path)
	} else if err != nil {
		file, _ = os.Create(path)
	}
	return file
}

func PrintLine(field string, value string, width int) {
	// Format field and value with alignment
	fieldFormatted := fmt.Sprintf("%-*s", width, field+":")
	valueFormatted := fmt.Sprint(value)
	fmt.Println(fieldColor(fieldFormatted) + attributeColor(valueFormatted))
}

// converts management policies to printable string
func ExtractPolicyNames(policies []v1.ManagementAction) string {
	policyNames := make([]string, len(policies))
	for i, policy := range policies {
		policyNames[i] = string(policy)
	}
	return strings.Join(policyNames, ", ")
}

// store key-value pairs in env-file
func StoreKeyValues(records map[string]string){
	envFile, err := os.Create(".xpcfi")
	kingpin.FatalIfError(err, "%s", errCreateCredEnvironment)
	defer envFile.Close()
	for field, attribute := range records{
		 _, err = envFile.WriteString(field + "=" + attribute + "\n")
		if err != nil{
			envFile.Close()
			kingpin.Fatalf("Could not store %s with value %s, %s",field, attribute, err)
		}
	}   
}

func UpdateTransactionID(){
	filename := ".xpcfi"
	transaction := uuid.New().String()
	input, err := os.ReadFile(filename)
    kingpin.FatalIfError(err, "Could not read file")

	lines := strings.Split(string(input), "\n")

	for i, line := range lines {
			if strings.Contains(line, "TRANSACTION_ID=") {
					lines[i] = "TRANSACTION_ID=" + transaction
			}
	}
	output := strings.Join(lines, "\n")
	err = os.WriteFile(filename, []byte(output), 0644)
	kingpin.FatalIfError(err, "Could not write file")
}