package main

import (
	"log"
	"time"

	"github.com/streadway/amqp" // Cliente oficial do RabbitMQ para Go
)

// dialRabbitMQ tenta conectar ao RabbitMQ com retries e backoff exponencial
func dialRabbitMQ(retries int, delay time.Duration) *amqp.Connection {
	var conn *amqp.Connection
	var err error

	for i := 0; i < retries; i++ {
		// Tenta se conectar ao RabbitMQ
		conn, err = amqp.Dial("amqp://guest:guest@rabbitmq:5672/")
		if err == nil {
			log.Printf("✅ Conectado ao RabbitMQ com sucesso após %d tentativas", i+1)
			return conn // Conexão bem-sucedida
		}

		// Loga o erro da tentativa atual
		log.Printf("❌ Erro ao conectar ao RabbitMQ (tentativa %d/%d): %v", i+1, retries, err)
		time.Sleep(delay) // Aguarda antes de tentar novamente
		delay *= 2        // Aplica backoff exponencial (2s, 4s, 8s, ...)
	}

	// Após todas as tentativas, encerra o programa com erro
	log.Fatalf("❌ Falha ao conectar ao RabbitMQ após %d tentativas: %v", retries, err)
	return nil
}

func main() {
	// Retry de até 5 tentativas com delay inicial de 2 segundos
	conn := dialRabbitMQ(5, 2*time.Second)
	defer conn.Close() // Garante fechamento da conexão ao encerrar o programa

	// Abre um canal de comunicação com o RabbitMQ
	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("❌ Erro ao abrir canal: %v", err)
	}
	defer ch.Close()

	// Declara a fila "ping_events" (deve ter o mesmo nome usado pelo producer/server)
	q, err := ch.QueueDeclare(
		"ping_events", // nome da fila
		false,         // durable: fila não persiste após reinício do broker
		false,         // autoDelete: não será deletada automaticamente
		false,         // exclusive: pode ser usada por outros consumidores
		false,         // noWait: aguarda resposta do broker
		nil,           // argumentos adicionais
	)
	if err != nil {
		log.Fatalf("Erro ao declarar fila: %v", err)
	}

	// Registra o consumidor para começar a escutar mensagens da fila
	msgs, err := ch.Consume(
		q.Name, // nome da fila
		"",     // consumer tag (vazio = gerado automaticamente)
		true,   // auto-ack: confirma automaticamente o recebimento da mensagem
		false,  // exclusive: permite múltiplos consumidores
		false,  // no-local: não usado na maioria dos casos
		false,  // noWait: aguarda o broker confirmar
		nil,    // argumentos adicionais
	)
	if err != nil {
		log.Fatalf("Erro ao registrar consumidor: %v", err)
	}

	log.Println("Worker aguardando mensagens...")

	// Canal usado para manter o processo vivo
	forever := make(chan bool)

	// Goroutine que processa as mensagens assim que chegam
	go func() {
		for d := range msgs {
			log.Printf("🛠️  Worker Processando: %s", d.Body) // Aqui poderia ser feito qualquer tipo de processamento
		}
	}()

	<-forever // Mantém o processo em execução indefinidamente
}
