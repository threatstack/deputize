FROM public.ecr.aws/lambda/provided:al2023

# Copy function handler executable
COPY bootstrap ${LAMBDA_RUNTIME_DIR}

# Set the CMD to the custom runtime executable
CMD [ "bootstrap" ]