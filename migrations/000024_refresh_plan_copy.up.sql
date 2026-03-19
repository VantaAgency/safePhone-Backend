UPDATE plans
SET
  device_range_fr = 'Smartphones jusqu''à 50 000 XOF',
  device_range_en = 'Smartphones up to 50,000 XOF',
  features_fr = '["Écran cassé ou fissuré après chute accidentelle","Panne électronique simple hors oxydation","Assistance standard pour la déclaration et le suivi","Exemples éligibles : itel A-series, TECNO Spark Go, Infinix Smart, Redmi A-series"]'::jsonb,
  features_en = '["Cracked or broken screen after an accidental drop","Basic electronic malfunction excluding oxidation","Standard assistance for claim filing and tracking","Eligible examples: itel A-series, TECNO Spark Go, Infinix Smart, Redmi A-series"]'::jsonb,
  not_covered_fr = '["Liquides, humidité et immersion","Vol, perte ou remplacement complet","Autres appareils du foyer ou téléphone non déclaré"]'::jsonb,
  not_covered_en = '["Liquids, humidity, and immersion","Theft, loss, or full replacement","Other household devices or undeclared phones"]'::jsonb
WHERE slug = 'essentiel';

UPDATE plans
SET
  device_range_fr = 'Smartphones de 50 000 à 150 000 XOF',
  device_range_en = 'Smartphones from 50,000 to 150,000 XOF',
  features_fr = '["Écran avant ou arrière cassé après choc ou chute","Panne électronique courante et petits dommages accidentels","Éclaboussures et contact liquide léger","Support prioritaire avec suivi accéléré","Exemples éligibles : TECNO Camon, Infinix Hot, Galaxy A0x/A1x, Redmi Note entrée de gamme, Oppo A-series"]'::jsonb,
  features_en = '["Broken front or back screen after impact or a drop","Common electronic issues and minor accidental damage","Splashes and light liquid contact","Priority support with faster tracking","Eligible examples: TECNO Camon, Infinix Hot, Galaxy A0x/A1x, entry Redmi Note models, Oppo A-series"]'::jsonb,
  not_covered_fr = '["Immersion complète, oxydation avancée ou appareil submergé","Vol, perte ou remplacement intégral du téléphone","Appareils supplémentaires du foyer non inclus"]'::jsonb,
  not_covered_en = '["Full immersion, advanced oxidation, or submerged devices","Theft, loss, or full device replacement","Additional household devices not included"]'::jsonb
WHERE slug = 'ecran-plus';

UPDATE plans
SET
  device_range_fr = 'Smartphones de 150 000 à 300 000 XOF',
  device_range_en = 'Smartphones from 150,000 to 300,000 XOF',
  features_fr = '["Écran cassé, chutes accidentelles et dommages du quotidien","Panne électronique et dysfonctionnement matériel","Dommages liés aux liquides, y compris immersion","Prise en charge prioritaire avec traitement plus rapide","Exemples éligibles : Galaxy A25/A35, iPhone 11/12, Redmi Note Pro, Oppo Reno Lite, TECNO Phantom"]'::jsonb,
  features_en = '["Broken screen, accidental drops, and everyday damage","Electronic failure and hardware malfunction","Liquid-related damage, including immersion","Priority handling with faster processing","Eligible examples: Galaxy A25/A35, iPhone 11/12, Redmi Note Pro, Oppo Reno Lite, TECNO Phantom"]'::jsonb,
  not_covered_fr = '["Vol ou perte","Remplacement intégral d''un smartphone haut de gamme","Autres appareils du foyer non déclarés"]'::jsonb,
  not_covered_en = '["Theft or loss","Full replacement of a high-end smartphone","Other undeclared household devices"]'::jsonb
WHERE slug = 'plus';

UPDATE plans
SET
  device_range_fr = 'Smartphones de 300 000 à 600 000 XOF',
  device_range_en = 'Smartphones from 300,000 to 600,000 XOF',
  features_fr = '["Casse écran, chocs et dommages accidentels importants","Panne électronique et dommages liquides complets","Vol et perte selon les conditions du contrat","Assistance express avec solution de remplacement si disponible","Exemples éligibles : Galaxy S-series, iPhone 13/14, Huawei P-series, Xiaomi haut de gamme, Oppo Find-series"]'::jsonb,
  features_en = '["Screen breakage, shocks, and major accidental damage","Electronic failure and full liquid damage","Theft and loss subject to contract terms","Express support with a replacement solution when available","Eligible examples: Galaxy S-series, iPhone 13/14, Huawei P-series, premium Xiaomi models, Oppo Find-series"]'::jsonb,
  not_covered_fr = '["Dommage intentionnel ou usage frauduleux","Appareil non enregistré ou accessoire seul","Équipements du foyer hors smartphone assuré"]'::jsonb,
  not_covered_en = '["Intentional damage or fraudulent use","Unregistered device or accessory only","Household equipment outside the insured smartphone"]'::jsonb
WHERE slug = 'haute';

UPDATE plans
SET
  device_range_fr = 'Smartphones dès 600 000 XOF et appareils du foyer déclarés',
  device_range_en = 'Smartphones from 600,000 XOF and declared household devices',
  features_fr = '["Protection du smartphone principal et des appareils du foyer prévus au contrat","Casse, panne, liquides, vol et perte sur les équipements couverts","Accompagnement premium avec traitement prioritaire","Remplacement ou solution de continuité selon disponibilité","Exemples éligibles : iPhone Pro/Pro Max, Galaxy S Ultra/Z, ordinateur portable, TV, tablette et électronique domestique déclarée"]'::jsonb,
  features_en = '["Protection for the main smartphone and household devices included in the contract","Breakage, malfunction, liquid damage, theft, and loss on covered equipment","Premium assistance with priority handling","Replacement or continuity solution depending on availability","Eligible examples: iPhone Pro/Pro Max, Galaxy S Ultra/Z, laptop, TV, tablet, and declared home electronics"]'::jsonb,
  not_covered_fr = '["Appareil non déclaré dans le périmètre du foyer","Dommage intentionnel, négligence grave ou fraude","Matériel professionnel ou commercial hors contrat"]'::jsonb,
  not_covered_en = '["Device not declared within the household scope","Intentional damage, gross negligence, or fraud","Professional or commercial equipment outside the contract"]'::jsonb
WHERE slug = 'totale';
