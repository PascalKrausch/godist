import urllib.request
import json
from pathlib import Path

LATITUDE = 51.7991
LONGITUDE = 10.6156
START_DATE = "2023-01-01"
END_DATE = "2025-12-31"

# Ohne CSV-Parameter liefert Open-Meteo standardmäßig JSON
URL = (
    f"https://archive-api.open-meteo.com/v1/archive?"
    f"latitude={LATITUDE}&longitude={LONGITUDE}&"
    f"start_date={START_DATE}&end_date={END_DATE}&"
    f"hourly=wind_speed_10m"
)

OUTPUT_FILE = Path(__file__).parent / "wind_2023_2025.json"

def download_wind_json():
    print(f"Lade Winddaten als JSON herunter...")
    req = urllib.request.Request(URL, headers={"User-Agent": "Godist-Example-Downloader"})
    
    with urllib.request.urlopen(req) as response:
        raw_data = response.read().decode("utf-8")
        
    parsed_json = json.loads(raw_data)
    
    with open(OUTPUT_FILE, "w", encoding="utf-8") as f:
        json.dump(parsed_json, f, indent=2)
        
    print(f"Gespeichert in {OUTPUT_FILE.resolve()} ({len(parsed_json['hourly']['wind_speed_10m'])} Datenpunkte).")

if __name__ == "__main__":
    download_wind_json()