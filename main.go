package main

import (
	"flag"
	"os"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"go.uber.org/zap"
)

var (
	vpcID            string
	availabilityZone string
	region           string
	duration         time.Duration
	logger           *zap.Logger
)

func init() {
	logger = zap.NewExample()
	defer logger.Sync()

	flag.StringVar(&region, "region", os.Getenv("AWS_REGION"), "AWS region")
	flag.StringVar(&vpcID, "vpc", "", "AWS vpc ID, required")
	flag.StringVar(&availabilityZone, "az", "", "AWS availability zone name, required")
	flag.DurationVar(&duration, "duration", time.Duration(60), "AWS availability zone network partiton duration (default 60s)")
}

func main() {
	flag.Parse()

	if vpcID == "" || availabilityZone == "" {
		logger.Fatal("must specipy vpcID and availabilityZone")
	}

	ec2Client := ec2.New(session.New())

	// list subnets by vpc ID and availability zone name
	subnets, err := ec2Client.DescribeSubnets(&ec2.DescribeSubnetsInput{
		Filters: []*ec2.Filter{
			{
				Name: aws.String("vpc-id"),
				Values: []*string{
					aws.String(vpcID),
				},
			},
			{
				Name: aws.String("availability-zone"),
				Values: []*string{
					aws.String(availabilityZone),
				},
			},
		},
	})
	if err != nil {
		logger.Fatal("DescribeSubnets", zap.Error(err))
	}

	if len(subnets.Subnets) == 0 {
		logger.Info("no subnets in vpc", zap.String("vpcID", vpcID), zap.String("availabilityZoneName", availabilityZone))
		return
	}

	logger.Info("DescribeSubnetsResult", zap.Any("Subnets", subnets.Subnets))

	// save original AclAssociationId and NetworkAclId mapping
	// AclAssociationId will used for replace network acl, AclAssociationId will updated when execute replace
	// save original NetworkAclId used for recovery
	originAclAssociations := make(map[string]string)
	for _, subnet := range subnets.Subnets {
		// list network acl by subnet id
		subnetID := subnet.SubnetId
		aclFilters := &ec2.DescribeNetworkAclsInput{
			Filters: []*ec2.Filter{
				{
					Name: aws.String("vpc-id"),
					Values: []*string{
						aws.String(vpcID),
					},
				},
				{
					Name: aws.String("association.subnet-id"),
					Values: []*string{
						subnetID,
					},
				},
			},
		}

		acls, err := ec2Client.DescribeNetworkAcls(aclFilters)
		if err != nil {
			logger.Fatal("DescribeNetworkAcls", zap.Error(err))
		}

		for _, acl := range acls.NetworkAcls {
			for _, ass := range acl.Associations {
				if *ass.SubnetId == *subnetID {
					originAclAssociations[*ass.NetworkAclAssociationId] = *ass.NetworkAclId
				}
			}
		}
	}

	logger.Info("DescribeNetworkAclsResult", zap.Any("originAclAssociations", originAclAssociations))

	acl, err := ec2Client.CreateNetworkAcl(&ec2.CreateNetworkAclInput{
		VpcId: aws.String(vpcID),
	})
	if err != nil {
		logger.Fatal("CreateNetworkAcl", zap.Error(err))
	}

	aclID := acl.NetworkAcl.NetworkAclId
	logger.Info("CreateNetworkAclResult", zap.String("aclID", *aclID))

	// The rule deny all out bound traffic to anywhere
	engress := &ec2.CreateNetworkAclEntryInput{
		CidrBlock:    aws.String("0.0.0.0/0"),
		Egress:       aws.Bool(true),
		NetworkAclId: aclID,
		PortRange: &ec2.PortRange{
			From: aws.Int64(53),
			To:   aws.Int64(53),
		},
		Protocol:   aws.String("-1"),
		RuleAction: aws.String("deny"),
		RuleNumber: aws.Int64(100),
	}
	_, err = ec2Client.CreateNetworkAclEntry(engress)
	if err != nil {
		logger.Fatal("CreateNetworkAclEntry", zap.Error(err))
	}

	// The rule deny all in bound traffic from anywhere
	ingress := &ec2.CreateNetworkAclEntryInput{
		CidrBlock:    aws.String("0.0.0.0/0"),
		Egress:       aws.Bool(false),
		NetworkAclId: aclID,
		PortRange: &ec2.PortRange{
			From: aws.Int64(53),
			To:   aws.Int64(53),
		},
		Protocol:   aws.String("-1"),
		RuleAction: aws.String("deny"),
		RuleNumber: aws.Int64(101),
	}

	_, err = ec2Client.CreateNetworkAclEntry(ingress)
	if err != nil {
		logger.Fatal("CreateNetworkAclEntry", zap.Error(err))
	}

	newAclAssociations := make(map[string]string)
	for assID, originAclID := range originAclAssociations {
		replacedNetworkAclAssociation, err := ec2Client.ReplaceNetworkAclAssociation(&ec2.ReplaceNetworkAclAssociationInput{
			AssociationId: aws.String(assID),
			NetworkAclId:  aclID,
		})
		if err != nil {
			logger.Fatal("ReplaceNetworkAclAssociation", zap.Error(err))
		}

		// save NewAssociationId and originAclID for recovery
		newAclAssociations[*replacedNetworkAclAssociation.NewAssociationId] = originAclID
	}

	logger.Info("inject availability zone network partition successful")
	logger.Info("sleep", zap.Duration("durtion", duration))
	time.Sleep(duration * time.Second)
	logger.Info("start recovery error injection")
	for newAssID, originAclID := range newAclAssociations {
		_, err := ec2Client.ReplaceNetworkAclAssociation(&ec2.ReplaceNetworkAclAssociationInput{
			AssociationId: aws.String(newAssID),
			NetworkAclId:  aws.String(originAclID),
		})
		if err != nil {
			logger.Fatal("ReplaceNetworkAclAssociation", zap.Error(err))
		}
	}

	logger.Info("recovery acl associations successful")
	_, err = ec2Client.DeleteNetworkAcl(&ec2.DeleteNetworkAclInput{
		NetworkAclId: aclID,
	})

	if err != nil {
		logger.Fatal("DeleteNetworkAcl", zap.Error(err))
	}
	logger.Info("recovery network acl successful")
}
