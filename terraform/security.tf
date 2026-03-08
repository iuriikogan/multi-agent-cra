# Cloud Armor Security Policy (Model Armor Implementation)
resource "google_compute_security_policy" "agent_armor" {
  name = "agent-armor-policy"

  # Rule 1: Allow specific traffic
  rule {
    action   = "allow"
    priority = "1000"
    match {
      versioned_expr = "SRC_IPS_V1"
      config {
        src_ip_ranges = ["*"]
      }
    }
    description = "Allow access"
  }

  # Rule 2: SQL Injection Protection
  rule {
    action   = "deny(403)"
    priority = "900"
    match {
      expr {
        expression = "evaluatePreconfiguredExpr('sqli-v33-stable')"
      }
    }
    description = "Block SQL Injection"
  }

  # Rule 3: Cross-Site Scripting Protection
  rule {
    action   = "deny(403)"
    priority = "901"
    match {
      expr {
        expression = "evaluatePreconfiguredExpr('xss-v33-stable')"
      }
    }
    description = "Block XSS"
  }

  # Rule 4: Model Armor / LLM Protection (Placeholder)
  # In a real Model Armor setup, this would reference specific AI protection rulesets
  # typically available in Cloud Armor Enterprise.
  # For now, we simulate this with a rule blocking known malicious prompts if signatures were available.
  rule {
    action   = "deny(403)"
    priority = "800"
    match {
      expr {
        expression = "evaluatePreconfiguredExpr('cve-canary')" # Placeholder for AI exploit rules
      }
    }
        description = "Block AI Exploits (Model Armor)"
      }
    
      # Default Rule (Required)
      rule {
        action   = "deny(403)"
        priority = "2147483647"
        match {
          versioned_expr = "SRC_IPS_V1"
          config {
            src_ip_ranges = ["*"]
          }
        }
        description = "Default deny all"
      }
    }
    