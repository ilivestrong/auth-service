# auth-service

## About this service

This service is written in `Golang` and uses `Connect Framework` to create the RPCs. The protos are compiled using `Buf`. The Go code interacts with other components like `PostgreSQL` database for persiting profiles and events and `RabbmitMQ` to allow auth-service to interact with otp-service in an event-driven fashion.

## RPCs implemented
- **SignupWithPhoneNumber** - This RPC allows a person to signup with their phone number. If successful, an event is sent to RabbitMQ exchange. The listener OTP service then consume the event and uses Twilio to send a 6 digit OTP sms to the phone number.  

- **VerifyPhoneNumber** - Upon receivng an OTP, this RPC can be used to verify the user's phone number and their OTP.  

- **LoginWithPhoneNumber** - Once phone number is verified, this RPC can be used to login into the service. Upon successful login, the service returns back a JWT auth token.  This token is required to invoke - "GetProfile" and "Logout" RPCs as they both are secured APIs.  

- **GetProfile** - A logged-in user can invoke this RPC to see their profile details. User must provide an authorization bearer header with token received in in Login step.  

- **Logout** - A user can end their session but invoking this RPC, this would invalidate the current JWT token.  


## Configure service dependencies

We first need to  configure below required components:

### Database
This service uses PostgreSQL database, so run an instance of PG database as standalone installation or inside docker container.  
As an example, below command can be used to run PostgreSQL container. *Make sure you have Docker installed locally*  

```sh
docker run --name postgresdb -v pg-data:/var/lib/postgresql/data -p 5432:5432 -e POSTGRES_PASSWORD=postgres -d postgres
```

`NOTE: Upon running PG instance, please create a database named 'midas'. If you want any other name, make sure to update it accordingly in the ".env" file as shown below in .env section.`  


### RabbitMQ
This service also uses RabbitMQ to produce and consume service events to communicate with otp-service. So run an instance preferably inside a docker container.  As an example, below command can be used to run RabbitMQ container. *Make sure you have Docker installed locally*  

```sh
docker run -it --rm --name rabbitmq -p 5672:5672 -p 15672:15672 rabbitmq:3.13-management
```

### .env  
Open the .env file in the root of the `auth-service` folder and enter below required PostgreSQL and RabbitMQ config details.  

`AMQP_ADDRESS` - This is RabbitMQ local running URL containing its host, user/password and port.  

`DB-*` - All keys starting with `DB-` are PostgreSQL details. The `Port` is the PG running port.  

`TOKEN_EXPIRY_IN_MINUTES` - This is validity `in minutes` of the token you generate in the Login step.

```sh
AMQP_ADDRESS=amqp://guest:guest@localhost:5672/
DB_HOST=localhost
DB_NAME=midas
DB_USERNAME=postgres
DB_PASSWORD=postgres
DB_PORT=5432
PORT=8080
TOKEN_EXPIRY_IN_MINUTES=20
```

## Run the service 
To run the service we need to install Go dependencies i.e., third-party packages used. CD into the root of the project directory. And run below commands sequentially:

**Install dependencies**
```sh
go mod download
```
**Then, run :**
```sh
go run main.go
```  

If everything setup correctly, you will see something like this:
![alt text](image.png)

