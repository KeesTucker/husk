# husk: Open-Source ECU Calibration for Motorbikes

husk is a free, open-source ECU calibration application designed for reverse-engineering and recalibrating modern motorbikes. This project is currently focused on teh Husqvarna 701 (2018).

### WARNING!

There are several important disclaimers when using husk:

- **ECU Modification Risk:** Modifying your ECU can be dangerous. Incorrect modifications may permanently damage the ECU or other components. Consult with a professional mechanic or tuner before making any changes. Use at your own risk.
- **Engine Damage:** Improper ECU calibration (e.g. Ignition Timing, Mass Airflow Calibration, Rev Limiter) can directly harm your motorbike’s engine. husk and its contributors are not responsible for any damage resulting from tool use.
- **Legal Compliance:** Recalibrating regulated systems (e.g., emissions) is illegal. husk does not provide definitions for modifying these systems. By using husk, you agree to abide by all local regulations regarding these calibrations.

## Current Focus

husk is currently being developed for the Husqvarna 701 but aims to expand support to more models and manufacturers once support for this model is completed. Feel free to contribute for your own motorbikes etc.

## Support
### Supported Hardware:
  | Device                     | Supported                                                                |
  |----------------------------|--------------------------------------------------------------------------|
  | Arduino with CANBUS Shield | ![In Progress](https://badgen.net/badge/color/In%20Progress/blue?label=) |

### Supported Motorbikes:
  | Make      | Model | Year      | Market |
  |-----------|-------|-----------|--------|
  | Husqvarna | 701   | 2014-2020 | Global |
  | KTM       | 690   | 2014-2020 | Global |

### Supported Features:
| Family          | Diagnostics                                                      | ROM Modification                                                 | Flashing                                                         |
|-----------------|------------------------------------------------------------------|------------------------------------------------------------------|------------------------------------------------------------------|
| KTM & Husqvarna | ![Planned](https://badgen.net/badge/color/Planned/purple?label=) | ![Planned](https://badgen.net/badge/color/Planned/purple?label=) | ![Planned](https://badgen.net/badge/color/Planned/purple?label=) |

## Emissions Statement

husk is open-source software intended for the legal modification of ECU calibrations. Users must adhere to all emissions regulations. husk does not support or facilitate the removal or tampering of any emissions-related components. Compliance with laws such as the Clean Air Act is critical.

Non-compliance could jeopardize husk and other projects. Use husk responsibly and legally.

## License

husk is licensed under the MIT License. This license permits code modification and functionality addition while promoting the use of husk for both personal and commercial purposes, as long as the original copyright notice is included.

## Project Setup & Building

To build husk from the source, follow these steps:

1. **Clone the Repository:**
   ```bash
   git clone https://github.com/KeesTucker/husk.git
   cd husk
   ```
2. **Set Up Your Environment:**
   Make sure you have [Go](https://golang.org/dl/) installed (Version 1.23.2 or later) and properly set up on your machine.
3. **Install Fyne:**
   Husk uses the Fyne framework for the UI. Please refer to the Fyne [docs]((https://docs.fyne.io/started/)) for installation instructions.
4. **Install Dependencies:**
   ```bash
   go mod tidy
   ```

5. **Compile the Code:**
   To build the project, run:
   ```bash
   go build -o husk.exe
   ```

6. **Run the Application:**
   After building, you can run the application using:
   ```bash
   ./husk.exe
   ```