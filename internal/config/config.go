package config

import (
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server ServerConfig `mapstructure:"server"`
	MySQL  MySQLConfig  `mapstructure:"mysql"`
	Redis  RedisConfig  `mapstructure:"redis"`
	Kafka  KafkaConfig  `mapstructure:"kafka"`
	Upload UploadConfig `mapstructure:"upload"`
	Auth   AuthConfig   `mapstructure:"auth"`
}

type ServerConfig struct {
	Port         int           `mapstructure:"port"`
	Mode         string        `mapstructure:"mode"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
}

type MySQLConfig struct {
	DSN          string `mapstructure:"dsn"`
	MaxIdleConns int    `mapstructure:"max_idle_conns"`
	MaxOpenConns int    `mapstructure:"max_open_conns"`
}

type RedisConfig struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type KafkaConfig struct {
	Brokers []string `mapstructure:"brokers"`
	Topic   string   `mapstructure:"topic"`
	GroupID string   `mapstructure:"group_id"`
	Enabled bool     `mapstructure:"enabled"`
}

type UploadConfig struct {
	ImageDir     string `mapstructure:"image_dir"`
	PublicPrefix string `mapstructure:"public_prefix"`
}

type AuthConfig struct {
	CompatibleMissingToken bool `mapstructure:"compatible_missing_token"`
}

// 加载配置
func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	v.SetEnvPrefix("HMDP")                             //环境变量加前缀 HMDP_
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_")) //把 key 里的 . 换成 _
	v.AutomaticEnv()                                   //每次 Get 都自动去查环境变量

	//提供兜底默认值
	v.SetDefault("server.port", 8080)
	v.SetDefault("server.mode", "debug")
	v.SetDefault("server.read_timeout", "10s")
	v.SetDefault("server.write_timeout", "10s")
	v.SetDefault("mysql.max_idle_conns", 10)
	v.SetDefault("mysql.max_open_conns", 50)
	v.SetDefault("redis.addr", "127.0.0.1:6379")
	v.SetDefault("redis.db", 1)
	v.SetDefault("kafka.topic", "kafka-orders")
	v.SetDefault("kafka.group_id", "my-kafka-group")
	v.SetDefault("kafka.enabled", true)
	v.SetDefault("upload.public_prefix", "/imgs")
	for _, key := range []string{
		"server.port",
		"server.mode",
		"server.read_timeout",
		"server.write_timeout",
		"mysql.dsn",
		"mysql.max_idle_conns",
		"mysql.max_open_conns",
		"redis.addr",
		"redis.password",
		"redis.db",
		"kafka.brokers",
		"kafka.topic",
		"kafka.group_id",
		"kafka.enabled",
		"upload.image_dir",
		"upload.public_prefix",
		"auth.compatible_missing_token",
	} {
		_ = v.BindEnv(key) //逐个(显式把) key 绑定到环境变量
	}

	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
