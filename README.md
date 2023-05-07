# google-photo-exporter-metadata-fixer

This is a simple command line tool that can be used to fix the metadata of the exported photos from Google Photos.

## Download

- [Windows](https://github.com/opdude/go-google-photo-exporter-metadata-fixer/releases/latest/google-photo-exporter-metadata-fixer-amd64-windows.exe)
- [Linux](https://github.com/opdude/go-google-photo-exporter-metadata-fixer/releases/latest/google-photo-exporter-metadata-fixer-amd64-linux)
- [MacOS](https://github.com/opdude/go-google-photo-exporter-metadata-fixer/releases/latest/google-photo-exporter-metadata-fixer-amd64-darwin)

## Usage

Pre-requisites:
- [Exported photos from Google Photos](https://takeout.google.com/settings/takeout/custom/photo)

If you only want to fix the metadata of the exported photos, you can run the following command:

```bash
google-photo-exporter-metadata-fixer <path-to-exported-photos>
```

If you'd also like to remove the JSON files that are created by the Google Photo Exporter, you can run the following command:

```bash
google-photo-exporter-metadata-fixer --deleteJSONFiles <path-to-exported-photos>
```
