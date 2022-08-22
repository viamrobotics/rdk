describe('should load the page', () => {
  it('passes', () => {
    cy.visit('http://127.0.0.1:8080/');

    cy.get('v-collapse.base').contains('h2', 'base').should('exist');
    cy.get('v-collapse.gantry').contains('h2', 'gantry').should('exist');
    cy.get('v-collapse.movement').contains('h2', 'movement').should('exist');
    cy.get('v-collapse.arm').contains('h2', 'arm').should('exist');
    cy.get('v-collapse.arm').contains('h2', 'arm').should('exist');
    cy.get('v-collapse.gripper').contains('h2', 'gripper').should('exist');
    cy.get('v-collapse.servo').contains('h2', 'servo').should('exist');
    cy.get('v-collapse.motor').contains('h2', 'motor').should('exist');
    cy.get('v-collapse.input').contains('h2', 'input').should('exist');
    cy.contains('h2', "WebGamepad").should('exist');
    cy.get('v-collapse.board').contains('h2', 'board').should('exist');
    cy.get('v-collapse.sensors').contains('h2', 'Sensors').should('exist');
    cy.get('v-collapse.navigation').contains('h2', 'Navigation Service').should('exist');
    cy.get('v-collapse.operations').contains('h2', 'Current Operations').should('exist');
    cy.get('v-collapse.camera').contains('h2', 'camera').should('exist');
    cy.get('v-collapse.do').contains('h2', 'Do()').should('exist');
  })
})