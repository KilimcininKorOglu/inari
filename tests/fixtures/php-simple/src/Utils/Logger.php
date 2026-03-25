<?php

namespace App\Utils;

/** Simple logging utility. */
class Logger
{
    private string $context;

    public function __construct(string $context)
    {
        $this->context = $context;
    }

    public function info(string $message): void
    {
        echo "[INFO] {$this->context}: {$message}\n";
    }

    public function error(string $message): void
    {
        fwrite(STDERR, "[ERROR] {$this->context}: {$message}\n");
    }

    public function debug(string $message): void
    {
        echo "[DEBUG] {$this->context}: {$message}\n";
    }
}
