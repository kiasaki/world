#!/bin/bash
set -euo pipefail

# Configuration
BUCKET_NAME="${1:-my-app-bucket}"
USER_NAME="${2:-s3-${BUCKET_NAME}-user}"
ROLE_NAME="${3:-s3-${BUCKET_NAME}-role}"
POLICY_NAME="s3-${BUCKET_NAME}-policy"

# Check prerequisites
command -v aws >/dev/null 2>&1 || { echo "aws cli not found"; exit 1; }

echo "=== Setting up S3 access for bucket: ${BUCKET_NAME} ==="
echo "User: ${USER_NAME}"
echo "Role: ${ROLE_NAME}"

# Create S3 bucket if it doesn't exist
echo -e "\n[1/5] Creating S3 bucket..."
if aws s3api head-bucket --bucket "${BUCKET_NAME}" 2>/dev/null; then
    echo "Bucket ${BUCKET_NAME} already exists"
else
    aws s3api create-bucket --bucket "${BUCKET_NAME}" \
        --region "${AWS_DEFAULT_REGION:-us-east-1}" \
        --output text
    echo "Created bucket ${BUCKET_NAME}"
fi

# Create IAM role (useful for EC2/Lambda assume role scenarios)
echo -e "\n[2/5] Creating IAM role..."
ROLE_ARN=$(aws iam get-role --role-name "${ROLE_NAME}" --query 'Role.Arn' --output text 2>/dev/null || true)
if [[ -n "${ROLE_ARN}" ]]; then
    echo "Role ${ROLE_NAME} already exists: ${ROLE_ARN}"
else
    ROLE_ARN=$(aws iam create-role \
        --role-name "${ROLE_NAME}" \
        --assume-role-policy-document '{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"Service":["ec2.amazonaws.com","lambda.amazonaws.com"]},"Action":"sts:AssumeRole"}]}' \
        --query 'Role.Arn' \
        --output text)
    echo "Created role: ${ROLE_ARN}"
fi

# Create inline policy for S3 read/write access
echo -e "\n[3/5] Creating IAM policy..."
POLICY_DOC=$(cat <<EOF
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "s3:ListBucket",
                "s3:GetBucketLocation"
            ],
            "Resource": "arn:aws:s3:::${BUCKET_NAME}"
        },
        {
            "Effect": "Allow",
            "Action": [
                "s3:GetObject",
                "s3:PutObject",
                "s3:DeleteObject",
                "s3:PutObjectAcl"
            ],
            "Resource": "arn:aws:s3:::${BUCKET_NAME}/*"
        }
    ]
}
EOF
)

# Attach policy to role
aws iam put-role-policy \
    --role-name "${ROLE_NAME}" \
    --policy-name "${POLICY_NAME}" \
    --policy-document "${POLICY_DOC}" >/dev/null
echo "Attached policy to role"

# Create IAM user
echo -e "\n[4/5] Creating IAM user..."
USER_ARN=$(aws iam get-user --user-name "${USER_NAME}" --query 'User.Arn' --output text 2>/dev/null || true)
if [[ -n "${USER_ARN}" ]]; then
    echo "User ${USER_NAME} already exists: ${USER_ARN}"
else
    USER_ARN=$(aws iam create-user \
        --user-name "${USER_NAME}" \
        --query 'User.Arn' \
        --output text)
    echo "Created user: ${USER_ARN}"
fi

# Attach policy to user
aws iam put-user-policy \
    --user-name "${USER_NAME}" \
    --policy-name "${POLICY_NAME}" \
    --policy-document "${POLICY_DOC}" >/dev/null
echo "Attached policy to user"

# Create access key
echo -e "\n[5/5] Creating access key..."
KEYS=$(aws iam create-access-key --user-name "${USER_NAME}" --output json)
ACCESS_KEY_ID=$(echo "${KEYS}" | jq -r '.AccessKey.AccessKeyId')
SECRET_ACCESS_KEY=$(echo "${KEYS}" | jq -r '.AccessKey.SecretAccessKey')

echo -e "\n=== Setup Complete ==="
echo -e "\nAccess Key ID: ${ACCESS_KEY_ID}"
echo "Secret Access Key: ${SECRET_ACCESS_KEY}"
echo -e "\nSave these credentials securely. Secret key cannot be retrieved again."
echo -e "\nTo configure AWS CLI with these credentials:"
echo "  aws configure --profile ${USER_NAME}"
echo -e "\nTo test access:"
echo "  aws s3 ls s3://${BUCKET_NAME} --profile ${USER_NAME}"
echo "  echo 'test' | aws s3 cp - s3://${BUCKET_NAME}/test.txt --profile ${USER_NAME}"
