# DynamiGo

DynamiGo, Go veritabanı sorgularını otomatik olarak gerçek tiplerle eşleşen dinamik struct'lara dönüştüren hafif bir kütüphanedir. Laravel'deki `stdClass` benzeri bir deneyimi Go'da sağlar, ancak Go'nun statik tip güvenliğinden ödün vermez.

## 🌟 Temel Özellikleri

- **Otomatik Tip Belirleme**: Veritabanı sorgusunun sonuçlarına göre doğru veri tipleriyle struct oluşturur
- **İç İçe Alan Desteği**: `address.city` gibi nokta notasyonuyla iç içe alanlara erişim
- **Doğrudan Kullanım**: Değerleri koşul ifadelerinde ve operasyonlarda doğrudan kullanabilirsiniz
- **Tipe Özel Getters**: `GetBool()`, `GetInt()`, `GetFloat()`, `GetString()` gibi tip güvenlikli getterlar
- **Saf Go Implementasyonu**: Harici bağımlılık gerektirmez, sadece standart kütüphane kullanır

## 🤔 Neden DynamiGo?

### Problem

Go'da veritabanı sorgularını çalıştırdığınızda, sonuçları almak için genellikle iki yol vardır:

1. **Önceden tanımlanmış struct**'lar: Esnek değildir, her sorgu için farklı struct yapıları gerektirir
2. **map[string]interface{}**: Oldukça esnek, ancak tip dönüşümü zorluğu yaratır

```go
// map[string]interface{} kullanımındaki zorluk:
data := map[string]interface{}{"active": true}

// Bu doğrudan çalışmaz - tip dönüşümü gerektirir
if data["active"] {  // Derleme hatası!
    // ...
}

// Bunun yerine her zaman tip dönüşümü yapmanız gerekir
if active, ok := data["active"].(bool); ok && active {
    // ...
}
```

### Çözüm

DynamiGo, veritabanı sorgularını otomatik olarak Go değerlerine dönüştürür ve tip güvenli bir API sağlar:

```go
// DynamiGo ile:
results, _ := dynamigo.QueryToStruct(db, "SELECT id, active, count FROM products")

// Doğrudan if koşullarında kullanım:
if results[0].GetBool("active") {
    fmt.Println("Ürün aktif!")
}

// Sayısal işlemler:
if results[0].GetInt("count") > 10 {
    fmt.Println("Yüksek stok!")
}
```

## 📦 Kurulum

```bash
go get github.com/yourusername/dynamigo
```

## 🚀 Kullanım

### Basit Sorgu

```go
package main

import (
    "database/sql"
    "fmt"
    "log"

    "github.com/yourusername/dynamigo"
    _ "github.com/lib/pq"
)

func main() {
    db, err := sql.Open("postgres", "connection-string")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    query := "SELECT id, title, active, price FROM products WHERE category_id = $1"

    results, err := dynamigo.QueryToStruct(db, query, 5)
    if err != nil {
        log.Fatal(err)
    }

    for _, product := range results {
        // Boolean değerler
        if product.GetBool("active") {
            // Sayısal değerler
            id := product.GetInt("id")
            price := product.GetFloat("price")

            // String değerler
            title := product.GetString("title")

            fmt.Printf("Ürün #%d: %s - %.2f TL\n", id, title, price)
        }
    }
}
```

### İç İçe Alanlarla Kullanım

```go
query := `SELECT
    p.id,
    p.title,
    p.price,
    c.name AS category.name,
    s.stock AS stock.quantity,
    s.status AS stock.status
FROM products p
JOIN categories c ON c.id = p.category_id
JOIN stock s ON s.product_id = p.id
WHERE p.id = $1`

result, err := dynamigo.QueryToStruct(db, query, 42)
if err != nil {
    log.Fatal(err)
}

if len(result) > 0 {
    product := result[0]

    fmt.Println("Ürün:", product.GetString("title"))
    fmt.Println("Kategori:", product.GetString("category.name"))

    // İç içe alanlara erişim
    if product.GetInt("stock.quantity") > 0 && product.GetString("stock.status") == "available" {
        fmt.Println("Stokta var!")
    }
}
```

## 💡 Laravel'den Go'ya Geçiş Yapanlar İçin

Laravel'de aşağıdaki yapıyı kullanıyorsanız:

```php
$products = DB::select('select * from products');
foreach ($products as $product) {
    if ($product->active) {
        echo $product->title;
    }
}
```

DynamiGo ile Go'da benzer şekilde yazabilirsiniz:

```go
products, _ := dynamigo.QueryToStruct(db, "SELECT * FROM products")
for _, product := range products {
    if product.GetBool("active") {
        fmt.Println(product.GetString("title"))
    }
}
```

## 🧪 Birim Testleri

Tüm testleri çalıştırmak için:

```bash
go test -v ./...
```

## 📄 Lisans

MIT Lisansı altında dağıtılmaktadır. Daha fazla bilgi için `LICENSE` dosyasına bakın.

## 🤝 Katkıda Bulunma

Katkılarınızı memnuniyetle karşılıyoruz! Lütfen bir pull request göndermeden önce testlerinizi ekleyin ve kodun Go standartlarına uygun olduğundan emin olun.

## 📊 Performans

DynamiGo, dinamik tiplerle çalıştığı ve reflection kullandığı için, önceden tanımlanmış struct'lara göre az miktarda performans farkı gösterebilir. Ancak, pek çok uygulama için bu fark ihmal edilebilir seviyededir ve DynamiGo'nun sağladığı esneklik bu ufak performans maliyetini fazlasıyla telafi eder.

## 🙏 İlham Kaynakları

Bu kütüphane, Laravel'in `stdClass` nesnesi ve PHP'nin dinamik tiplemesi gibi özelliklerin Go'da güvenli bir şekilde kullanılabilmesi için geliştirilmiştir. Go'nun statik tip sisteminin avantajlarını korurken, dinamik dillerdeki esnekliği sunmayı amaçlar.
