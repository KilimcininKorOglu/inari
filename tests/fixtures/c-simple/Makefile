CC = gcc
CFLAGS = -Wall -Wextra

all: main

main: src/controller.c src/payments.c src/utils.c
	$(CC) $(CFLAGS) -o main $^

clean:
	rm -f main
