package main

import (
	"flag"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/elbv2"
)

const (
	condkey   = "path-pattern"
	condvalue = "/*"
	bluetag   = "blue"
	greentag  = "green"
)

var (
	region *string
	albARN *string
)

func main() {
	// Input parameters
	region = flag.String("region", "eu-central-1", "Region to search for ALB")
	albARN = flag.String("albarn", "", "ARN of the ALB where we look for active environment")
	flag.Parse()

	// Get and configure session and service
	sess := session.Must(session.NewSession())
	sess.Config.Region = region
	elbSVC := elbv2.New(sess)

	// Create listener request
	listenerReq := &elbv2.DescribeListenersInput{
		LoadBalancerArn: albARN,
	}

	// Request for listener arn
	dl, err := elbSVC.DescribeListeners(listenerReq)
	if err != nil {
		panic(err)
	}

	// We expect one listener. If this is not the case exit
	if len(dl.Listeners) > 1 {
		panic("We got more listeners than expected! Exiting...")
	}

	// Create rule request
	ruleReq := &elbv2.DescribeRulesInput{
		ListenerArn: dl.Listeners[0].ListenerArn,
	}

	// Request for rules
	dr, err := elbSVC.DescribeRules(ruleReq)
	if err != nil {
		panic(err)
	}

	// Iterate rules
	var targetGroupArn string
	for _, rule := range dr.Rules {
		for _, cond := range rule.Conditions {
			if *cond.Field == condkey && *cond.Values[0] == condvalue && len(rule.Actions) == 1 {
				targetGroupArn = *rule.Actions[0].TargetGroupArn
			}
		}
	}

	if targetGroupArn == "" {
		panic("Couldn't find live target group arn. First deployment?")
	}

	// Create tg tags request
	tgTagsReq := &elbv2.DescribeTagsInput{
		ResourceArns: []*string{
			&targetGroupArn,
		},
	}

	// Request for target group tags
	req, tgOut := elbSVC.DescribeTagsRequest(tgTagsReq)
	err = req.Send()
	if err != nil {
		panic(err)
	}

	// Iterate tags
	for _, t := range tgOut.TagDescriptions[0].Tags {
		if *t.Key == "Name" {
			// It is blue because we found blue in the name
			v := strings.ToLower(*t.Value)
			if strings.Contains(v, bluetag) {
				fmt.Println(bluetag)
				return
			} else if strings.Contains(v, greentag) {
				fmt.Println(greentag)
			}
		}
	}

	// Found nothing. Error!
	panic("Couldnt find blue or green in target group Name tag. Exiting...")
}
