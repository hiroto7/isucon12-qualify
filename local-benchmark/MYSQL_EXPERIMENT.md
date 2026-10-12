# SQLiteをMySQLへ移す追試

## 結果

APIで使うテナントDBをMySQLへ全面移行した候補は **103,150、エラー0**。同じバイナリ・同じMySQL設定でSQLiteを使う対照は **142,942、エラー0**。候補は対照の72.16%（約27.84%低下）で、不採用。SQLiteの採用済みソースと元のMySQL設定へ戻した。

これはこの移行案の結果であり、SQLiteを外すすべての設計案が遅い、または現在の実装が性能限界に達したという結論ではない。DB間の通信、テナント単位の接続・ロック、サーバ内の処理のどれが主因かは今回の計測だけでは確定していない。

| 試行 | 条件 | スコア | エラー |
| --- | --- | ---: | ---: |
| 034 | 前回の採用済みSQLite、元のMySQL設定 | 147,252 | 0 |
| 035 | Dockerのmissing parent snapshotで配備準備失敗、ベンチ未実行 | 無効 | — |
| 036 | 候補の同じバイナリ、backend=SQLite、MySQL 1GiB/1024接続 | 142,942 | 0 |
| 037 | 同じバイナリ、backend=MySQL、同じMySQL設定 | 103,150 | 0 |
| 038 | SQLiteと元のMySQL設定へ復元後 | 144,735 | 0 |

VMのapp 2 vCPU/4GiB、bench 4 vCPU/8GiBを維持。公式ベンチのstrictPrepare・60秒・配点・シナリオも維持。034と036の差は約2.93%で、037の低下を元の古いスコアとの比較だけから判断していない。036/037はComposeのbackendフラグ以外の全アプリソースハッシュと実行バイナリSHA256が一致し、どちらもMySQLのbuffer pool 1GiB、max_connections 1024。InnoDBのコミット同期設定は緩めていない。

## 実装した移行候補

- 各テナントを別のMySQL databaseに分離し、APIのplayer/competition/player_scoreの読取・書込をすべてMySQLへ移した。既存の課金・IDもMySQL。SQLiteは初期データ移行の入力としてのみ使用。
- utf8mb4_0900_bin（NO PAD）で、IDの大小文字・末尾空白の比較をSQLiteに合わせた。
- ランキング・最終スコア・課金の版管理とトランザクション、既存のロックを維持。MySQLではCSV/参加者追加を200行ずつパラメータ付き複数行INSERTにし、単純移植の行単位通信往復を避けた。
- 100テナントの15,903 players、1,196 competitions、3,725,307 player_score、計3,742,406行を移行。各行の全列を正規化して照合し、全テーブルのSHA256と行数が元のSQLiteに一致。監査記録は[こちら](validation/mysql-tenant-import-audit.json)。
- 初期化は正常停止したMySQL全体の2.9GiBスナップショットを、guest loopback専用の管理サービスで停止→コピー→起動し、DB readyを待つ。予備実測8.22秒、公式037中8.85秒で初期化期限を通過した。ベンチコードを変えたり、初期データを縮小したりしていない。

## 検証と復元

本番コンテナで実MySQLのテストを含むrace suiteとgo vetを通過。Unicode・引用符・backslash・NULのテキスト、ID比較、テナント分離、最終行・同点順位、ページ、310円の課金、200行を超えるバッチの途中失敗で全体をロールバック、cold cache、空CSV、別大会の保持、失格を検証した。

037の公式ベンチはエラー0。実際にbackend=mysqlのプロセス環境と274本のテナントDB接続を確認した。SQLiteの*.dbを開いているFDは確認されなかった。

その後、実blackauth/HTTPS APIで合成テナント・参加者・大会・CSVを作成。app VMを通常再起動し、次の初期化前にランキング・詳細・210円課金の応答一致、失格・権限・テナント分離、新IDの非重複を確認した。保存データは[再起動前](validation/mysql-reboot-data-before.json)、[起動後のサービス状態](validation/mysql-reboot-after.txt)。ブラウザ画面の目視や電源断耐久性は検証していない。

不採用後はアプリの全Go/SQL/Composeをccd78e7と同じ状態へ戻し、追加したMySQL設定・実験テナントDB・権限を削除、snapshotサービスを停止・無効化した。復元後の公式ベンチ038の結果と、ソース/バイナリ・設定ハッシュを残している。候補のソースは[再適用用patch](experiments/mysql-tenants.patch)として保持し、本番の採用済み実装には入れていない。

## 再現のための記録

基点のアプリソースはccd78e74242c687f1c93b9a616fc468fe0aebc08。候補patchを適用し、Goファイルを含む完全な配備を行う。構築は通常のVM管理操作と元の公式データだけを使う。

1. guestでappを止め、SQLiteモードのsql/init.shでadminを初期化。
2. mysql-tenant-experiment.cnfをguestへ入れ、MySQLを再起動。scripts/import-mysql-tenants.pyをrootで実行し、全行照合を完了させる。
3. isuconアカウントへ実験用t12_tenant_のdatabase権限を付与。MySQLを正常停止し、/var/lib/mysqlを/var/lib/mysql-isucon12-seedへコピーして.seed-readyを作成する。
4. scripts/mysql-snapshot-service.pyとmysql-snapshot.serviceをguestに設置し、loopback管理サービスを有効化。通常の初期化データを復元できることを確認。
5. 同じMySQL設定のままbackendフラグを切り替えて対照/候補を測る。試行中の配備を避け、全試行を保存する。

サービスとseedは隔離guest専用。停止/コピー/起動を行うrootサービスをホストへ設置していない。ISUCON12の講評・解説・参加記は今回も参照していない。
