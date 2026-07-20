param(
  [Parameter(Mandatory = $true)]
  [string[]] $Path
)

$ErrorActionPreference = "Stop"

function Write-CertificateDetails {
  param(
    [Parameter(Mandatory = $true)]
    [string] $Label,

    $Certificate
  )

  if ($null -eq $Certificate) {
    Write-Host "  ${Label}: <none>"
    return
  }

  Write-Host "  ${Label} subject: $($Certificate.Subject)"
  Write-Host "  ${Label} issuer: $($Certificate.Issuer)"
  Write-Host "  ${Label} thumbprint: $($Certificate.Thumbprint)"
  Write-Host "  ${Label} valid from: $($Certificate.NotBefore)"
  Write-Host "  ${Label} valid until: $($Certificate.NotAfter)"
}

foreach ($file in $Path) {
  $signature = Get-AuthenticodeSignature -FilePath $file

  Write-Host "Authenticode signature for ${file}:"
  Write-Host "  Status: $($signature.Status)"
  if ($signature.StatusMessage) {
    Write-Host "  StatusMessage: $($signature.StatusMessage)"
  }
  Write-CertificateDetails -Label "Signer" -Certificate $signature.SignerCertificate
  Write-CertificateDetails -Label "Timestamp" -Certificate $signature.TimeStamperCertificate

  if ($signature.Status -eq "Valid") {
    continue
  }

  if ($null -eq $signature.SignerCertificate) {
    throw "No Authenticode signer certificate found for ${file}: $($signature.Status)"
  }

  throw "Invalid Authenticode signature for ${file}: $($signature.Status)"
}
